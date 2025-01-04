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
	"errors"
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

func (u *User) OnStart(_ context.Context) error {
	fmt.Println("User OnStart")
	return nil
}

func (u *User) OnStop(_ context.Context) error {
	fmt.Println("User OnStop")
	return nil
}

func (u *User) TaskSet() taskset.TaskSet {
	return taskset.NewRandom(
		taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
			fmt.Println("User Task 1")
			return nil
		}),
		NewTaskSet(),
	)
}

type TaskSet struct {
	*taskset.Sequential
}

func NewTaskSet() taskset.TaskSet {
	return &TaskSet{
		Sequential: taskset.NewSequential(
			taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
				fmt.Println("TaskSet Task 1")
				return nil
			}),
			taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
				fmt.Println("TaskSet Task 2")
				return errors.New("error")
			}),
			taskset.TaskFunc(func(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
				fmt.Println("TaskSet Task 3")
				return nil
			}),
		),
	}
}

func (s *TaskSet) OnStart(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	fmt.Println("TaskSet OnStart")
	return nil
}

func (s *TaskSet) OnStop(_ context.Context, _ launce.User) error {
	fmt.Println("TaskSet OnStop")
	return nil
}
