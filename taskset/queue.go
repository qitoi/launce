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

type taskQueue struct {
	tasks   []Task
	current Task
}

func (tq *taskQueue) Empty() bool {
	return len(tq.tasks) == 0
}

func (tq *taskQueue) Next() Task {
	t := tq.tasks[0]
	if len(tq.tasks) == 1 {
		tq.tasks = tq.tasks[:0]
	} else {
		tq.tasks = tq.tasks[1:]
	}
	tq.current = t
	return t
}

func (tq *taskQueue) Current() Task {
	return tq.current
}

func (tq *taskQueue) Schedule(t Task, first bool) {
	if first {
		tq.tasks = append([]Task{t}, tq.tasks...)
	} else {
		tq.tasks = append(tq.tasks, t)
	}
}
