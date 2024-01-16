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
	"testing"

	"github.com/qitoi/launce/taskset"
)

type dummyTask struct {
	taskset.Task
}

func (d dummyTask) Unwrap() taskset.Task {
	return d.Task
}

func TestGetWeight(t *testing.T) {
	testcases := []struct {
		Name     string
		Task     taskset.Task
		Expected int
	}{
		{
			"Single Weight Task",
			taskset.Weight(nil, 10),
			10,
		},
		{
			"Nested Weight Task",
			taskset.Weight(taskset.Weight(nil, 20), 30),
			30,
		},
		{
			"Weight Task in Dummy Task",
			dummyTask{taskset.Weight(nil, 40)},
			40,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			weight := taskset.GetWeight(testcase.Task)
			if weight != testcase.Expected {
				t.Fatalf("unexpected weight. got:%v want:%v", weight, testcase.Expected)
			}
		})
	}
}
