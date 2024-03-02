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
	_ Task = (*weightedTask)(nil)
)

type weightedTask struct {
	Task
	weight int
}

func (w *weightedTask) Unwrap() Task {
	return w.Task
}

// Weight returns a task with weight for random taskset.
func Weight(task Task, weight int) Task {
	return &weightedTask{
		Task:   task,
		weight: weight,
	}
}

func GetWeight(task Task) int {
	for task != nil {
		if wt, ok := task.(*weightedTask); ok {
			return wt.weight
		}
		if ut, ok := task.(interface{ Unwrap() Task }); ok {
			task = ut.Unwrap()
		} else {
			break
		}
	}
	return 0
}
