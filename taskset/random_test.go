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
	"testing"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/taskset"
)

func TestRandom_Run(t *testing.T) {
	testcases := []struct {
		Name    string
		Weights []int
		Num     int
	}{
		{Name: "TestCase #1", Weights: []int{1, 1}, Num: 100000},
		{Name: "TestCase #2", Weights: []int{1, 1, 1}, Num: 100000},
		{Name: "TestCase #3", Weights: []int{5, 2, 1}, Num: 100000},
		{Name: "TestCase #4", Weights: []int{4, 2, 1, 1}, Num: 100000},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			var result []int
			var tasks []taskset.Task

			for i, weight := range testcase.Weights {
				i, weight := i, weight
				tasks = append(
					tasks,
					taskset.Weight(
						taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
							result = append(result, i)
							if len(result) == testcase.Num {
								return launce.StopUser
							}
							return nil
						}),
						weight,
					),
				)
			}

			var random taskset.TaskSet = taskset.NewRandom(tasks...)
			err := taskset.Run(context.Background(), random, nil)

			if !errors.Is(err, launce.StopUser) {
				t.Fatalf("unexpected error. got:%v, want:%v", err, launce.StopUser)
			}

			taskCount := make([]int, len(testcase.Weights))
			for _, n := range result {
				taskCount[n] += 1
			}

			for i, n := range taskCount {
				if n == 0 {
					t.Fatalf("uncalled task #%v", i)
				}
			}
		})
	}
}
