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

import (
	"context"
	"math/rand"

	"github.com/qitoi/launce"
)

var (
	_ TaskSet = (*Random)(nil)
)

type Random struct {
	tasks    []Task
	filtered []Task
}

func NewRandom(tasks ...Task) *Random {
	r := &Random{}
	r.Init(tasks)
	return r
}

func (r *Random) Init(tasks []Task) {
	totalWeight := 0
	for _, t := range tasks {
		totalWeight += max(GetWeight(t), 1)
	}
	r.tasks = make([]Task, totalWeight)

	idx := 0
	for _, t := range tasks {
		weight := max(GetWeight(t), 1)
		for i := 0; i < weight; i++ {
			r.tasks[idx] = t
			idx += 1
		}
	}

	r.filtered = r.tasks
}

func (r *Random) Len() int {
	return len(r.filtered)
}

func (r *Random) Next() Task {
	return r.filtered[rand.Intn(len(r.filtered))]
}

func (r *Random) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(0)
}

func (r *Random) OnStart(ctx context.Context, s Scheduler) error {
	return nil
}

func (r *Random) OnStop(ctx context.Context) error {
	return nil
}

func (r *Random) ApplyFilter(opts ...FilterOption) {
	r.filtered = FilterTasks(r.tasks, opts...)
}

func (r *Random) Run(ctx context.Context, u launce.User, s Scheduler) error {
	return Run(ctx, r, u)
}
