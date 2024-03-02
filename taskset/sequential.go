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

	"github.com/qitoi/launce"
)

var (
	_ TaskSet = (*Sequential)(nil)
)

// Sequential is a taskset that processes tasks in order.
type Sequential struct {
	tasks    []Task
	filtered []Task
	index    int
}

// NewSequential returns a new Sequential TaskSet.
func NewSequential(tasks ...Task) *Sequential {
	s := &Sequential{}
	s.Init(tasks)
	return s
}

func (s *Sequential) Init(tasks []Task) {
	s.tasks = tasks
	s.filtered = tasks
}

func (s *Sequential) Len() int {
	return len(s.filtered)
}

func (s *Sequential) Next() Task {
	idx := s.index
	s.index = (s.index + 1) % len(s.filtered)
	return s.filtered[idx]
}

func (s *Sequential) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(0)
}

func (s *Sequential) OnStart(ctx context.Context, _ Scheduler) error {
	return nil
}

func (s *Sequential) OnStop(ctx context.Context) error {
	return nil
}

func (s *Sequential) FilterTasks(f func(tasks []Task) []Task) {
	s.filtered = f(s.tasks)
}

func (s *Sequential) Run(ctx context.Context, u launce.User, _ Scheduler) error {
	return Run(ctx, s, u)
}
