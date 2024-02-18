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
	"fmt"
	"slices"
	"testing"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/taskset"
)

func TestSequential_Run(t *testing.T) {
	testcases := []struct {
		Name     string
		Tasks    []func(n int) error
		Expected []int
	}{
		{
			Name: "Stop User #1",
			Tasks: []func(n int) error{
				func(n int) error {
					return nil
				},
				func(n int) error {
					return nil
				},
				func(n int) error {
					return launce.StopUser
				},
			},
			Expected: []int{0, 1, 2},
		},
		{
			Name: "Stop User #2",
			Tasks: []func(n int) error{
				func(n int) error {
					return nil
				},
				func(n int) error {
					return launce.StopUser
				},
				func(n int) error {
					return nil
				},
			},
			Expected: []int{0, 1},
		},
		{
			Name: "Loop TaskSet",
			Tasks: []func(n int) error{
				func(n int) error {
					return nil
				},
				func(n int) error {
					return nil
				},
				func(n int) error {
					if n >= 6 {
						return launce.StopUser
					}
					return nil
				},
			},
			Expected: []int{0, 1, 2, 0, 1, 2},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			var result []int
			var tasks []taskset.Task

			for i, task := range testcase.Tasks {
				i, task := i, task
				tasks = append(tasks, taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
					result = append(result, i)
					return task(len(result))
				}))
			}

			seq := taskset.NewSequential(tasks...)
			err := taskset.Run(context.Background(), seq, nil)

			if !errors.Is(err, launce.StopUser) {
				t.Fatalf("unexpected error. got:%v, want:%v", err, launce.StopUser)
			}

			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v, want:%v", result, testcase.Expected)
			}
		})
	}
}

func TestSequential_ApplyFilter(t *testing.T) {
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
			Name:        "include tag1",
			Tags:        tags("tag1"),
			ExcludeTags: nil,
			Expected:    []int{1},
		},
		{
			Name:        "include tag100",
			Tags:        tags("tag100"),
			ExcludeTags: nil,
			Expected:    []int{100},
		},
		{
			Name:        "include tag2, tag3, exclude tag2",
			Tags:        tags("tag2", "tag3"),
			ExcludeTags: tags("tag2"),
			Expected:    []int{3},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			var result []int

			var tasks []taskset.Task
			for i := 1; i <= 100; i++ {
				no := i
				tasks = append(tasks, taskset.Tag(taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
					result = append(result, no)
					return taskset.InterruptTaskSet
				}), fmt.Sprintf("tag%d", no)))
			}

			var opts []taskset.FilterOption
			if testcase.Tags != nil {
				opts = append(opts, taskset.IncludeTags(*testcase.Tags...))
			}
			if testcase.ExcludeTags != nil {
				opts = append(opts, taskset.ExcludeTags(*testcase.ExcludeTags...))
			}

			seq := taskset.NewSequential(tasks...)
			seq.ApplyFilter(opts...)

			err := taskset.Run(context.Background(), seq, nil)

			if !errors.Is(err, taskset.RescheduleTask) {
				t.Fatal(err)
			}

			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v, want:%v", result, testcase.Expected)
			}
		})
	}
}
