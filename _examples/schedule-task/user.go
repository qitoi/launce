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
	"fmt"
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
	return launce.Constant(1 * time.Second)
}

func (u *User) TaskSet() taskset.TaskSet {
	return taskset.NewSequential(
		taskset.TaskFunc(u.seq1),
		taskset.TaskFunc(u.seq2),
	)
}

func (u *User) seq1(_ context.Context, _ launce.User, s taskset.Scheduler) error {
	fmt.Println("task seq1")

	// schedule extra tasks
	s.Schedule(taskset.TaskFunc(extra1), true)
	s.Schedule(taskset.TaskFunc(extra2), true)
	s.Schedule(taskset.TaskFunc(extra3), false)

	// task execution order:
	// 1. extra2
	// 2. extra1
	// 3. extra3
	// 4. seq2

	return nil
}

func (u *User) seq2(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	fmt.Println("task seq2")
	return taskset.InterruptTaskSet
}

func extra1(ctx context.Context, user launce.User, s taskset.Scheduler) error {
	fmt.Println("task extra1")
	return nil
}

func extra2(ctx context.Context, user launce.User, s taskset.Scheduler) error {
	fmt.Println("task extra2")
	return nil
}

func extra3(ctx context.Context, user launce.User, s taskset.Scheduler) error {
	fmt.Println("task extra3")
	return nil
}
