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

type User struct {
	taskset.User
}

func (u *User) Init(r launce.Runner, waitTime launce.WaitTimeFunc) {
	u.User.Init(r, waitTime)
	u.SetTaskSet(taskset.NewRandom(
		taskset.Weight(taskset.TaskFunc(foo), 1),
		taskset.Weight(taskset.TaskFunc(u.bar), 2),
		taskset.Weight(taskset.NewSequential(
			taskset.TaskFunc(u.seq1),
			taskset.TaskFunc(u.seq2),
			&seq3{},
		), 1),
	))
}

func (u *User) WaitTime() launce.WaitTimeFunc {
	return launce.Between(100*time.Millisecond, 200*time.Millisecond)
}

func foo(ctx context.Context, user launce.User) error {
	user.Report(
		http.MethodGet,
		"/foo",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return nil
}

func (u *User) bar(ctx context.Context, user launce.User) error {
	u.Report(
		http.MethodGet,
		"/bar",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return nil
}

func (u *User) seq1(ctx context.Context, user launce.User) error {
	u.Report(
		http.MethodGet,
		"/seq/1",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return nil
}

func (u *User) seq2(ctx context.Context, user launce.User) error {
	u.Report(
		http.MethodGet,
		"/seq/2",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return nil
}

type seq3 struct{}

func (s *seq3) Run(ctx context.Context, user launce.User) error {
	user.Report(
		http.MethodGet,
		"/seq/3",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return taskset.InterruptTaskSet
}
