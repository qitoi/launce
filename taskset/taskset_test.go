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

func TestTaskSet_NestTaskSet(t *testing.T) {
	testcases := []struct {
		Name          string
		Tasks         [][]func(n int) error
		Expected      []int
		ExpectedError error
	}{
		{
			Name: "Interrupt TaskSet",
			Tasks: [][]func(n int) error{
				{
					func(n int) error {
						return nil
					},
					func(n int) error {
						return taskset.InterruptTaskSet
					},
				},
				{
					func(n int) error {
						return taskset.RescheduleTask
					},
					func(n int) error {
						return taskset.InterruptTaskSetImmediately
					},
				},
				{
					func(n int) error {
						return taskset.RescheduleTaskImmediately
					},
					func(n int) error {
						return launce.StopUser
					},
				},
			},
			Expected:      []int{11, 12, 21, 22, 31, 32},
			ExpectedError: launce.StopUser,
		},
		{
			Name: "Stop User",
			Tasks: [][]func(n int) error{
				{
					func(n int) error {
						return nil
					},
					func(n int) error {
						return taskset.InterruptTaskSet
					},
				},
				{
					func(n int) error {
						return launce.StopUser
					},
					func(n int) error {
						return taskset.InterruptTaskSet
					},
				},
				{
					func(n int) error {
						return nil
					},
					func(n int) error {
						return launce.StopUser
					},
				},
			},
			Expected:      []int{11, 12, 21},
			ExpectedError: launce.StopUser,
		},
		{
			Name: "Loop Sub TaskSet",
			Tasks: [][]func(n int) error{
				{
					func(n int) error {
						return taskset.RescheduleTask
					},
					func(n int) error {
						if n >= 4 {
							return taskset.InterruptTaskSet
						}
						return nil
					},
				},
				{
					func(n int) error {
						return launce.StopUser
					},
				},
			},
			Expected:      []int{11, 12, 11, 12, 21},
			ExpectedError: launce.StopUser,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			var tasksets []taskset.Task
			var result []int
			for tasksetIdx, tasksetTasks := range testcase.Tasks {
				var tasks []taskset.Task
				for taskIdx, task := range tasksetTasks {
					id := (tasksetIdx+1)*10 + taskIdx + 1
					task := task
					tasks = append(tasks, taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
						result = append(result, id)
						return task(len(result))
					}))
				}
				seq := taskset.NewSequential(tasks...)
				tasksets = append(tasksets, seq)
			}

			root := taskset.NewSequential(tasksets...)
			err := root.Run(context.Background(), nil)

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
	testcases = nil

	interruptTaskSetTask := taskset.Tag(taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
		return taskset.InterruptTaskSet
	}), "stop")

	for _, testcase := range testcases {
		var result []int

		tasks := []taskset.Task{
			taskset.Tag(taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
				result = append(result, 1)
				return nil
			}), "tag1"),
			taskset.Tag(taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
				result = append(result, 2)
				return nil
			}), "tag2", "tag3"),
			taskset.NewSequential(
				taskset.Tag(taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
					result = append(result, 11)
					return nil
				}), "tag1", "tag3"),
				taskset.Tag(taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
					result = append(result, 12)
					return nil
				}), "tag2"),
				interruptTaskSetTask,
			),
			taskset.Tag(
				taskset.NewSequential(
					taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
						result = append(result, 21)
						return nil
					}),
					taskset.Tag(taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
						result = append(result, 22)
						return nil
					}), "tag3"),
					interruptTaskSetTask,
				),
				"tag4",
			),
			interruptTaskSetTask,
		}

		var opts []taskset.FilterOption
		if testcase.Tags != nil {
			opts = append(opts, taskset.IncludeTags(*testcase.Tags...))
		}
		if testcase.ExcludeTags != nil {
			opts = append(opts, taskset.ExcludeTags(*testcase.ExcludeTags...))
		}

		t.Run(testcase.Name, func(t *testing.T) {
			root := taskset.NewSequential(taskset.FilterTasks(tasks, opts...)...)

			if err := root.Run(context.Background(), nil); !errors.Is(err, taskset.RescheduleTask) {
				t.Fatal(err)
			}

			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v, want:%v", result, testcase.Expected)
			}
		})
	}

	t.Run("exclude taskset all tasks", func(t *testing.T) {
		var result []int
		expected := []int{1}

		tasks := []taskset.Task{
			taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
				result = append(result, 1)
				return nil
			}),
			taskset.NewSequential(
				taskset.Tag(
					taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
						result = append(result, 11)
						return nil
					}),
					"exclude",
				),
			),
			interruptTaskSetTask,
		}

		tasks = taskset.FilterTasks(tasks, taskset.ExcludeTags("exclude"))
		root := taskset.NewSequential(tasks...)
		if err := root.Run(context.Background(), nil); !errors.Is(err, taskset.RescheduleTask) {
			t.Fatal(err)
		}

		if slices.Compare(result, expected) != 0 {
			t.Fatalf("unexpected result. got:%v, want:%v", result, expected)
		}
	})

	t.Run("include taskset, exclude taskset all tasks", func(t *testing.T) {
		var result []int
		expected := []int{1}

		tasks := []taskset.Task{
			taskset.Tag(taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
				result = append(result, 1)
				return nil
			}), "include"),
			taskset.Tag(
				taskset.NewSequential(
					taskset.Tag(taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
						result = append(result, 11)
						return nil
					}), "exclude"),
				),
				"include",
			),
			taskset.Tag(interruptTaskSetTask, "include"),
		}

		tasks = taskset.FilterTasks(tasks, taskset.IncludeTags("include"), taskset.ExcludeTags("exclude"))
		root := taskset.NewSequential(tasks...)
		if err := root.Run(context.Background(), nil); !errors.Is(err, taskset.RescheduleTask) {
			t.Fatal(err)
		}

		if slices.Compare(result, expected) != 0 {
			t.Fatalf("unexpected result. got:%v, want:%v", result, expected)
		}
	})
}
