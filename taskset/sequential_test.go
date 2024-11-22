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

func TestSequential_Rerun(t *testing.T) {
	errUnknown := errors.New("unknown error")

	testcases := []struct {
		Name     string
		Tasks    []func() error
		Result   error
		Expected []int
	}{
		{
			Name: "Re-run Sequential TaskSet (StopUser)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return launce.StopUser
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
			Expected: []int{0, 1, 0, 1},
		},
		{
			Name: "Re-run Sequential TaskSet (RescheduleTask)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return taskset.RescheduleTask
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
			Expected: []int{0, 1, 2, 0, 1, 2},
		},
		{
			Name: "Re-run Sequential TaskSet (RescheduleTaskImmediately)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return taskset.RescheduleTaskImmediately
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
			Expected: []int{0, 1, 2, 0, 1, 2},
		},
		{
			Name: "Re-run Sequential TaskSet (InterruptTaskSet)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return taskset.InterruptTaskSet
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   taskset.RescheduleTask,
			Expected: []int{0, 1, 0, 1},
		},
		{
			Name: "Re-run Sequential TaskSet (InterruptTaskSetImmediately)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return taskset.InterruptTaskSetImmediately
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   taskset.RescheduleTaskImmediately,
			Expected: []int{0, 1, 0, 1},
		},
		{
			Name: "Re-run Sequential TaskSet (Unknown Error)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return errUnknown
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
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
					return task()
				}))
			}

			seq := taskset.NewSequential(tasks...)
			_ = taskset.Run(context.Background(), seq, nil)
			err := taskset.Run(context.Background(), seq, nil)

			if !errors.Is(err, testcase.Result) {
				t.Fatalf("unexpected error. got:%v, want:%v", err, testcase.Result)
			}

			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v, want:%v", result, testcase.Expected)
			}
		})
	}
}

func TestSequential_RerunNestedTaskSet(t *testing.T) {
	errUnknown := errors.New("unknown error")

	testcases := []struct {
		Name     string
		Tasks    []func() error
		Result   error
		Expected []int
	}{
		{
			Name: "Re-run Nested Sequential TaskSet (StopUser)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return launce.StopUser
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
			Expected: []int{0, 1, 0, 1},
		},
		{
			Name: "Re-run Nested Sequential TaskSet (RescheduleTask)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return taskset.RescheduleTask
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
			Expected: []int{0, 1, 2, 0, 1, 2},
		},
		{
			Name: "Re-run Nested Sequential TaskSet (RescheduleTaskImmediately)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return taskset.RescheduleTaskImmediately
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
			Expected: []int{0, 1, 2, 0, 1, 2},
		},
		{
			Name: "Re-run Nested Sequential TaskSet (InterruptTaskSet)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return taskset.InterruptTaskSet
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
			Expected: []int{0, 1, 0, 1},
		},
		{
			Name: "Re-run Nested Sequential TaskSet (InterruptTaskSetImmediately)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return taskset.InterruptTaskSetImmediately
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
			Expected: []int{0, 1, 0, 1},
		},
		{
			Name: "Re-run Nested Sequential TaskSet (Unknown Error)",
			Tasks: []func() error{
				func() error {
					return nil
				},
				func() error {
					return errUnknown
				},
				func() error {
					return launce.StopUser
				},
			},
			Result:   launce.StopUser,
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
					return task()
				}))
			}

			childSeq := taskset.NewSequential(tasks...)
			seq := taskset.NewSequential(
				childSeq,
				taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
					return launce.StopUser
				}),
			)
			_ = taskset.Run(context.Background(), seq, nil)
			err := taskset.Run(context.Background(), seq, nil)

			if !errors.Is(err, testcase.Result) {
				t.Fatalf("unexpected error. got:%v, want:%v", err, testcase.Result)
			}

			if slices.Compare(result, testcase.Expected) != 0 {
				t.Fatalf("unexpected result. got:%v, want:%v", result, testcase.Expected)
			}
		})
	}
}
