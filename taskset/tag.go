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

package taskset

var (
	_ Task = (*taggedTask)(nil)
)

type taggedTask struct {
	Task
	Tags []string
}

func (t *taggedTask) Unwrap() Task {
	return t.Task
}

func GetTags(task Task) []string {
	for task != nil {
		if tagged, ok := task.(*taggedTask); ok {
			return tagged.Tags
		}
		if ut, ok := task.(interface{ Unwrap() Task }); ok {
			task = ut.Unwrap()
		} else {
			break
		}
	}
	return nil
}

func Tag(task Task, tags ...string) Task {
	t := GetTags(task)
	return &taggedTask{
		Task: task,
		Tags: append(t, tags...),
	}
}
