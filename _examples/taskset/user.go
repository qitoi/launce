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

package main

import (
	"context"
	"net/http"
	"time"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/taskset"
)

var (
	_ taskset.User = (*User)(nil)
)

type User struct {
	taskset.UserImpl
}

func (u *User) WaitTime() launce.WaitTimeFunc {
	return launce.Between(100*time.Millisecond, 200*time.Millisecond)
}

func (u *User) TaskSet() taskset.TaskSet {
	return taskset.NewRandom(
		taskset.Weight(taskset.TaskFunc(task1), 1),
		taskset.Weight(taskset.TaskFunc(u.task2), 2),
		taskset.Weight(taskset.NewSequential(
			taskset.TaskFunc(u.seq1),
			taskset.TaskFunc(u.seq2),
			&seq3{},
		), 1),
	)
}

func task1(_ context.Context, u launce.User, _ taskset.Scheduler) error {
	u.Report(http.MethodGet, "/task1", launce.NoneResponseTime, 0, nil)
	return nil
}

func (u *User) task2(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	u.Report(http.MethodGet, "/task2", launce.NoneResponseTime, 0, nil)
	return nil
}

func (u *User) seq1(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	u.Report(http.MethodGet, "/seq/1", launce.NoneResponseTime, 0, nil)
	return nil
}

func (u *User) seq2(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	u.Report(http.MethodGet, "/seq/2", launce.NoneResponseTime, 0, nil)
	return nil
}

type seq3 struct{}

func (s *seq3) Run(_ context.Context, u launce.User, _ taskset.Scheduler) error {
	u.Report(http.MethodGet, "/seq/3", launce.NoneResponseTime, 0, nil)
	return taskset.InterruptTaskSet
}
