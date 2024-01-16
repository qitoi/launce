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
	"slices"
	"testing"

	"github.com/qitoi/launce/taskset"
)

func TestGetTags(t *testing.T) {
	testcases := []struct {
		Name     string
		Task     taskset.Task
		Expected []string
	}{
		{
			"Single Tagged Task",
			taskset.Tag(nil, "tag1"),
			[]string{"tag1"},
		},
		{
			"Multiple Tagged Task",
			taskset.Tag(nil, "tag1", "tag2"),
			[]string{"tag1", "tag2"},
		},
		{
			"Weight Task in Dummy Task",
			dummyTask{taskset.Tag(nil, "tag1", "tag2")},
			[]string{"tag1", "tag2"},
		},
		{
			"Nested Tagged Task #1",
			taskset.Tag(taskset.Tag(nil, "tag1"), "tag2"),
			[]string{"tag1", "tag2"},
		},
		{
			"Nested Tagged Task #2",
			taskset.Tag(dummyTask{taskset.Tag(nil, "tag1")}, "tag2"),
			[]string{"tag1", "tag2"},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			tags := taskset.GetTags(testcase.Task)
			if slices.Compare(tags, testcase.Expected) != 0 {
				t.Fatalf("unexpected tags. got:%v want:%v", tags, testcase.Expected)
			}
		})
	}
}
