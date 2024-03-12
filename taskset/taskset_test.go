/*
 *  Copyright 2024 qitoi
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package taskset_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/taskset"
)

var (
	_ launce.BaseUser = (*testUser)(nil)
	_ taskset.TaskSet = (*testTaskSet)(nil)
)

type testUser struct {
	launce.BaseUserImpl
	Result []int
	Cancel context.CancelFunc
}

func (t *testUser) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(0)
}

func (t *testUser) Process(_ context.Context) error {
	return nil
}

func (t *testUser) Add(n int) {
	t.Result = append(t.Result, n)
}

type testTaskSet struct {
	*taskset.Sequential
	onStart func(ctx context.Context, u launce.User, s taskset.Scheduler) error
	onStop  func(ctx context.Context, u launce.User) error
}

func (t *testTaskSet) OnStart(ctx context.Context, u launce.User, s taskset.Scheduler) error {
	if t.onStart != nil {
		return t.onStart(ctx, u, s)
	}
	return nil
}

func (t *testTaskSet) OnStop(ctx context.Context, u launce.User) error {
	if t.onStop != nil {
		return t.onStop(ctx, u)
	}
	return nil
}

func newTaskSet(onStart func(ctx context.Context, u launce.User, s taskset.Scheduler) error, onStop func(ctx context.Context, u launce.User) error, tasks ...taskset.Task) *testTaskSet {
	t := &testTaskSet{
		Sequential: taskset.NewSequential(tasks...),
		onStart:    onStart,
		onStop:     onStop,
	}
	return t
}

func buildNestedTaskSet(tasks [][]taskset.Task) taskset.TaskSet {
	var subTaskset []taskset.Task
	for _, ts := range tasks {
		subTaskset = append(subTaskset, newTaskSet(nil, nil, ts...))
	}
	return newTaskSet(nil, nil, subTaskset...)
}

func runTaskSet(task taskset.TaskSet) ([]int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	u := &testUser{
		Cancel: cancel,
	}
	err := taskset.Run(ctx, task, u)
	return u.Result, err
}

func TestTaskSet_NestTaskSet(t *testing.T) {
	testcases := []struct {
		Name          string
		Tasks         [][]taskset.Task
		Expected      []int
		ExpectedError error
	}{
		{
			Name: "Interrupt TaskSet",
			Tasks: [][]taskset.Task{
				{
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(11)
						return nil
					}),
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(12)
						return taskset.InterruptTaskSet
					}),
				},
				{
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return nil
					}),
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(22)
						return taskset.InterruptTaskSetImmediately
					}),
				},
				{
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(31)
						return taskset.RescheduleTaskImmediately
					}),
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(32)
						return launce.StopUser
					}),
				},
			},
			Expected:      []int{11, 12, 21, 22, 31, 32},
			ExpectedError: launce.StopUser,
		},
		{
			Name: "Stop User",
			Tasks: [][]taskset.Task{
				{
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(11)
						return nil
					}),
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(12)
						return taskset.InterruptTaskSet
					}),
				},
				{
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return launce.StopUser
					}),
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(22)
						return taskset.InterruptTaskSet
					}),
				},
				{
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(31)
						return nil
					}),
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(32)
						return launce.StopUser
					}),
				},
			},
			Expected:      []int{11, 12, 21},
			ExpectedError: launce.StopUser,
		},
		{
			Name: "Loop Sub TaskSet",
			Tasks: [][]taskset.Task{
				{
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(11)
						return taskset.RescheduleTask
					}),
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(12)
						if len(u.(*testUser).Result) >= 4 {
							return taskset.InterruptTaskSet
						}
						return nil
					}),
				},
				{
					taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return launce.StopUser
					}),
				},
			},
			Expected:      []int{11, 12, 11, 12, 21},
			ExpectedError: launce.StopUser,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			ts := buildNestedTaskSet(testcase.Tasks)
			result, err := runTaskSet(ts)

			if !errors.Is(err, testcase.ExpectedError) {
				t.Fatalf("unexpected error. got:%v want:%v", err, testcase.ExpectedError)
			}

			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v want:%v", result, testcase.Expected)
			}
		})
	}
}

func TestTaskSet_Events(t *testing.T) {
	var result []int

	ts := newTaskSet(
		func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
			result = append(result, 0)
			return nil
		},
		func(ctx context.Context, u launce.User) error {
			result = append(result, 9)
			return nil
		},
		newTaskSet(
			func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				result = append(result, 10)
				return nil
			},
			func(ctx context.Context, u launce.User) error {
				result = append(result, 19)
				return nil
			},
			newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					result = append(result, 100)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					result = append(result, 109)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					result = append(result, 105)
					return taskset.InterruptTaskSet
				}),
			),
			taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				result = append(result, 15)
				return taskset.InterruptTaskSet
			}),
		),
		newTaskSet(
			func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				result = append(result, 20)
				return nil
			},
			func(ctx context.Context, u launce.User) error {
				result = append(result, 29)
				return nil
			},
			newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					result = append(result, 200)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					result = append(result, 209)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					result = append(result, 205)
					return taskset.InterruptTaskSet
				}),
			),
			taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				result = append(result, 25)
				return taskset.InterruptTaskSet
			}),
		),
		taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
			result = append(result, 5)
			return taskset.InterruptTaskSet
		}),
	)

	_ = taskset.Run(context.Background(), ts, nil)

	expected := []int{0, 10, 100, 105, 109, 15, 19, 20, 200, 205, 209, 25, 29, 5, 9}
	if slices.Compare(result, expected) != 0 {
		t.Fatalf("unexpected result. got:%v, want:%v", result, expected)
	}
}

func TestFilterTasks(t *testing.T) {
	tags := func(tags ...string) *[]string {
		return &tags
	}

	testcases := []struct {
		Name        string
		Tags        *[]string
		ExcludeTags *[]string
		Expected    []int
	}{
		{
			Name:        "empty tags",
			Tags:        nil,
			ExcludeTags: nil,
			Expected:    []int{1, 2, 11, 12, 21, 22},
		},
		{
			Name:        "include stop tag",
			Tags:        tags("stop"),
			ExcludeTags: nil,
			Expected:    []int{},
		},
		{
			Name:        "include tag1, stop",
			Tags:        tags("tag1", "stop"),
			ExcludeTags: nil,
			Expected:    []int{1, 11},
		},
		{
			Name:        "include tag2, tag3, stop",
			Tags:        tags("tag2", "tag3", "stop"),
			ExcludeTags: nil,
			Expected:    []int{2, 11, 12, 22},
		},
		{
			Name:        "include tag4, stop",
			Tags:        tags("tag4", "stop"),
			ExcludeTags: nil,
			Expected:    []int{21, 22},
		},
		{
			Name:        "include tag3, tag4, stop",
			Tags:        tags("tag3", "tag4", "stop"),
			ExcludeTags: nil,
			Expected:    []int{2, 11, 21, 22},
		},
		{
			Name:        "exclude tag1",
			Tags:        nil,
			ExcludeTags: tags("tag1"),
			Expected:    []int{2, 12, 21, 22},
		},
		{
			Name:        "exclude tag3",
			Tags:        nil,
			ExcludeTags: tags("tag3"),
			Expected:    []int{1, 12, 21},
		},
		{
			Name:        "exclude tag1, tag2",
			Tags:        nil,
			ExcludeTags: tags("tag1", "tag2"),
			Expected:    []int{21, 22},
		},
		{
			Name:        "include tag1, stop, exclude tag1",
			Tags:        tags("tag1", "stop"),
			ExcludeTags: tags("tag1"),
			Expected:    []int{},
		},
		{
			Name:        "include tag1, stop, exclude tag2",
			Tags:        tags("tag1", "stop"),
			ExcludeTags: tags("tag2"),
			Expected:    []int{1, 11},
		},
		{
			Name:        "include tag1, stop, exclude tag3",
			Tags:        tags("tag1", "stop"),
			ExcludeTags: tags("tag3"),
			Expected:    []int{1},
		},
		{
			Name:        "include tag4, stop, exclude tag3",
			Tags:        tags("tag4", "stop"),
			ExcludeTags: tags("tag3"),
			Expected:    []int{21},
		},
	}

	interruptTaskSetTask := taskset.Tag(taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
		return taskset.InterruptTaskSet
	}), "stop")

	tasks := []taskset.Task{
		taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
			u.(*testUser).Add(1)
			return nil
		}), "tag1"),
		taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
			u.(*testUser).Add(2)
			return nil
		}), "tag2", "tag3"),
		newTaskSet(
			nil,
			nil,
			taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
				u.(*testUser).Add(11)
				return nil
			}), "tag1", "tag3"),
			taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
				u.(*testUser).Add(12)
				return nil
			}), "tag2"),
			interruptTaskSetTask,
		),
		taskset.Tag(
			newTaskSet(
				nil,
				nil,
				taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
					u.(*testUser).Add(21)
					return nil
				}),
				taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
					u.(*testUser).Add(22)
					return nil
				}), "tag3"),
				interruptTaskSetTask,
			),
			"tag4",
		),
		interruptTaskSetTask,
	}

	for _, testcase := range testcases {
		var opts []taskset.FilterOption
		if testcase.Tags != nil {
			opts = append(opts, taskset.IncludeTags(*testcase.Tags...))
		}
		if testcase.ExcludeTags != nil {
			opts = append(opts, taskset.ExcludeTags(*testcase.ExcludeTags...))
		}

		t.Run(testcase.Name, func(t *testing.T) {
			ts := newTaskSet(nil, nil, taskset.FilterTasks(tasks, opts...)...)

			result, err := runTaskSet(ts)

			if !errors.Is(err, taskset.RescheduleTask) {
				t.Fatal(err)
			}

			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v, want:%v", result, testcase.Expected)
			}
		})
	}

	t.Run("include taskset, exclude taskset all tasks", func(t *testing.T) {
		expected := []int{1}

		tasks := []taskset.Task{
			taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
				u.(*testUser).Add(1)
				return nil
			}), "include"),
			taskset.Tag(
				newTaskSet(nil, nil,
					taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
						u.(*testUser).Add(11)
						return nil
					}), "exclude"),
				),
				"include",
			),
			taskset.Tag(interruptTaskSetTask, "include"),
		}

		tasks = taskset.FilterTasks(tasks, taskset.IncludeTags("include"), taskset.ExcludeTags("exclude"))
		ts := newTaskSet(nil, nil, tasks...)

		result, err := runTaskSet(ts)

		if !errors.Is(err, taskset.RescheduleTask) {
			t.Fatal(err)
		}

		if slices.Compare(result, expected) != 0 {
			t.Fatalf("unexpected result. got:%v, want:%v", result, expected)
		}
	})
}

func TestFilterTasks_ExcludeAllSubTasks(t *testing.T) {
	interruptTaskSetTask := taskset.Tag(taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
		return taskset.InterruptTaskSet
	}), "stop")

	expected := []int{1}

	tasks := []taskset.Task{
		taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
			u.(*testUser).Add(1)
			return nil
		}),
		newTaskSet(nil, nil,
			taskset.Tag(
				taskset.TaskFunc(func(_ context.Context, u launce.User, _ taskset.Scheduler) error {
					u.(*testUser).Add(11)
					return nil
				}),
				"exclude",
			),
		),
		interruptTaskSetTask,
	}

	tasks = taskset.FilterTasks(tasks, taskset.ExcludeTags("exclude"))
	var ts taskset.TaskSet = newTaskSet(nil, nil, tasks...)

	result, err := runTaskSet(ts)

	if !errors.Is(err, taskset.RescheduleTask) {
		t.Fatal(err)
	}

	if slices.Compare(result, expected) != 0 {
		t.Fatalf("unexpected result. got:%v, want:%v", result, expected)
	}
}

func TestTaskSet_Schedule(t *testing.T) {
	testcases := []struct {
		Name     string
		TaskSet  taskset.TaskSet
		Expected []int
	}{
		{
			Name: "Schedule #1",
			TaskSet: newTaskSet(
				nil,
				nil,
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(11)
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return nil
					}), true)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(12)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					return taskset.InterruptTaskSet
				}),
			),
			Expected: []int{11, 21, 12},
		},
		{
			Name: "Schedule #2",
			TaskSet: newTaskSet(
				nil,
				nil,
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(11)
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return nil
					}), false)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(12)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					return taskset.InterruptTaskSet
				}),
			),
			Expected: []int{11, 21, 12},
		},
		{
			Name: "Schedule #3",
			TaskSet: newTaskSet(
				nil,
				nil,
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(11)
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return nil
					}), true)
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(22)
						return nil
					}), true)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(12)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					return taskset.InterruptTaskSet
				}),
			),
			Expected: []int{11, 22, 21, 12},
		},
		{
			Name: "Schedule #4",
			TaskSet: newTaskSet(
				nil,
				nil,
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(11)
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return nil
					}), false)
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(22)
						return nil
					}), false)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(12)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					return taskset.InterruptTaskSet
				}),
			),
			Expected: []int{11, 21, 22, 12},
		},
		{
			Name: "Schedule #5",
			TaskSet: newTaskSet(
				nil,
				nil,
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(11)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(12)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return nil
					}), true)
					return taskset.InterruptTaskSet
				}),
			),
			Expected: []int{11, 12},
		},
		{
			Name: "Schedule #6",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(21)
						return nil
					}), true)
					s.Schedule(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(22)
						return nil
					}), false)
					return nil
				},
				nil,
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(11)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(12)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					return taskset.InterruptTaskSet
				}),
			),
			Expected: []int{21, 22, 11, 12},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			result, err := runTaskSet(testcase.TaskSet)

			if !errors.Is(err, taskset.RescheduleTask) {
				t.Fatal(err)
			}

			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v, want:%v", result, testcase.Expected)
			}
		})
	}
}

func TestTaskSet_Cancel(t *testing.T) {
	testcases := []struct {
		Name     string
		TaskSet  taskset.TaskSet
		Expected []int
	}{
		{
			Name: "Cancel TaskSet Task #1",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					u.(*testUser).Cancel()
					return ctx.Err()
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(2)
					return nil
				}),
			),
			Expected: []int{0, 1, 9},
		},
		{
			Name: "Cancel TaskSet Task #2",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					u.(*testUser).Cancel()
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(2)
					return nil
				}),
			),
			Expected: []int{0, 1, 9},
		},
		{
			Name: "Cancel TaskSet OnStart #1",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					u.(*testUser).Cancel()
					return ctx.Err()
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(2)
					return nil
				}),
			),
			Expected: []int{0},
		},
		{
			Name: "Cancel TaskSet OnStart #2",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					u.(*testUser).Cancel()
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(2)
					return nil
				}),
			),
			Expected: []int{0},
		},
		{
			Name: "Cancel TaskSet OnStop #1",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					u.(*testUser).Cancel()
					return ctx.Err()
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(2)
					return taskset.InterruptTaskSet
				}),
			),
			Expected: []int{0, 1, 2, 9},
		},
		{
			Name: "Cancel TaskSet OnStop #2",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					u.(*testUser).Cancel()
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					return nil
				}),
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(2)
					return taskset.InterruptTaskSet
				}),
			),
			Expected: []int{0, 1, 2, 9},
		},
		{
			Name: "Cancel SubTaskSet Task",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					return nil
				}),
				newTaskSet(
					func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(10)
						return nil
					},
					func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(19)
						return nil
					},
					taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(11)
						u.(*testUser).Cancel()
						return ctx.Err()
					}),
				),
			),
			Expected: []int{0, 1, 10, 11, 19, 9},
		},
		{
			Name: "Cancel SubTaskSet OnStart",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					return nil
				}),
				newTaskSet(
					func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(10)
						u.(*testUser).Cancel()
						return ctx.Err()
					},
					func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(19)
						return nil
					},
					taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(11)
						return taskset.InterruptTaskSet
					}),
				),
			),
			Expected: []int{0, 1, 10, 9},
		},
		{
			Name: "Cancel SubTaskSet OnStop",
			TaskSet: newTaskSet(
				func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(0)
					return nil
				},
				func(ctx context.Context, u launce.User) error {
					u.(*testUser).Add(9)
					return nil
				},
				taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
					u.(*testUser).Add(1)
					return nil
				}),
				newTaskSet(
					func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(10)
						return nil
					},
					func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(19)
						u.(*testUser).Cancel()
						return ctx.Err()
					},
					taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
						u.(*testUser).Add(11)
						return taskset.InterruptTaskSet
					}),
				),
			),
			Expected: []int{0, 1, 10, 11, 19, 9},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			result, _ := runTaskSet(testcase.TaskSet)
			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v, want:%v", result, testcase.Expected)
			}
		})
	}
}
