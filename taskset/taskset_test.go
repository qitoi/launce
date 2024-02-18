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

type testUser struct {
	launce.BaseUser
	Result []int
}

func (t *testUser) Process(ctx context.Context) error {
	return nil
}

func (t *testUser) Add(n int) {
	t.Result = append(t.Result, n)
}

func buildNestedTaskSet(tasks [][]taskset.Task) taskset.TaskSet {
	var subTaskset []taskset.Task
	for _, ts := range tasks {
		subTaskset = append(subTaskset, taskset.NewSequential(ts...))
	}
	return taskset.NewSequential(subTaskset...)
}

func runTaskSet(task taskset.TaskSet) ([]int, error) {
	u := &testUser{}
	err := taskset.Run(context.Background(), task, u)
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
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(11)
						return nil
					}),
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(12)
						return taskset.InterruptTaskSet
					}),
				},
				{
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(21)
						return nil
					}),
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(22)
						return taskset.InterruptTaskSetImmediately
					}),
				},
				{
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(31)
						return taskset.RescheduleTaskImmediately
					}),
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
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
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(11)
						return nil
					}),
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(12)
						return taskset.InterruptTaskSet
					}),
				},
				{
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(21)
						return launce.StopUser
					}),
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(22)
						return taskset.InterruptTaskSet
					}),
				},
				{
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(31)
						return nil
					}),
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
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
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(11)
						return taskset.RescheduleTask
					}),
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
						u.(*testUser).Add(12)
						if len(u.(*testUser).Result) >= 4 {
							return taskset.InterruptTaskSet
						}
						return nil
					}),
				},
				{
					taskset.TaskFunc(func(ctx context.Context, u launce.User) error {
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

	interruptTaskSetTask := taskset.Tag(taskset.TaskFunc(func(_ context.Context, _ launce.User) error {
		return taskset.InterruptTaskSet
	}), "stop")

	tasks := []taskset.Task{
		taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User) error {
			u.(*testUser).Add(1)
			return nil
		}), "tag1"),
		taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User) error {
			u.(*testUser).Add(2)
			return nil
		}), "tag2", "tag3"),
		taskset.NewSequential(
			taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User) error {
				u.(*testUser).Add(11)
				return nil
			}), "tag1", "tag3"),
			taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User) error {
				u.(*testUser).Add(12)
				return nil
			}), "tag2"),
			interruptTaskSetTask,
		),
		taskset.Tag(
			taskset.NewSequential(
				taskset.TaskFunc(func(_ context.Context, u launce.User) error {
					u.(*testUser).Add(21)
					return nil
				}),
				taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User) error {
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
			ts := taskset.NewSequential(taskset.FilterTasks(tasks, opts...)...)

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
			taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User) error {
				u.(*testUser).Add(1)
				return nil
			}), "include"),
			taskset.Tag(
				taskset.NewSequential(
					taskset.Tag(taskset.TaskFunc(func(_ context.Context, u launce.User) error {
						u.(*testUser).Add(11)
						return nil
					}), "exclude"),
				),
				"include",
			),
			taskset.Tag(interruptTaskSetTask, "include"),
		}

		tasks = taskset.FilterTasks(tasks, taskset.IncludeTags("include"), taskset.ExcludeTags("exclude"))
		ts := taskset.NewSequential(tasks...)

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
	interruptTaskSetTask := taskset.Tag(taskset.TaskFunc(func(_ context.Context, _ launce.User) error {
		return taskset.InterruptTaskSet
	}), "stop")

	expected := []int{1}

	tasks := []taskset.Task{
		taskset.TaskFunc(func(_ context.Context, u launce.User) error {
			u.(*testUser).Add(1)
			return nil
		}),
		taskset.NewSequential(
			taskset.Tag(
				taskset.TaskFunc(func(_ context.Context, u launce.User) error {
					u.(*testUser).Add(11)
					return nil
				}),
				"exclude",
			),
		),
		interruptTaskSetTask,
	}

	tasks = taskset.FilterTasks(tasks, taskset.ExcludeTags("exclude"))
	ts := taskset.NewSequential(tasks...)

	result, err := runTaskSet(ts)

	if !errors.Is(err, taskset.RescheduleTask) {
		t.Fatal(err)
	}

	if slices.Compare(result, expected) != 0 {
		t.Fatalf("unexpected result. got:%v, want:%v", result, expected)
	}
}
