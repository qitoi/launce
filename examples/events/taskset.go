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

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/taskset"
)

var (
	_ taskset.TaskSet = (*TaskSet)(nil)
	_ taskset.TaskSet = (*SubTaskSet)(nil)
)

type TaskSet struct {
	*taskset.Random
}

func NewTaskSet() *TaskSet {
	t := &TaskSet{
		Random: taskset.NewRandom(
			taskset.Weight(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				fmt.Println("TaskSet task1")
				return nil
			}), 3),
			taskset.Weight(NewSubTaskSet(), 2),
			taskset.Weight(taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				fmt.Println("TaskSet quit")
				return taskset.InterruptTaskSet
			}), 1),
		),
	}
	return t
}

func (t *TaskSet) OnStart(ctx context.Context, u launce.User, s taskset.Scheduler) error {
	if err := t.Random.OnStart(ctx, u, s); err != nil {
		return err
	}
	fmt.Println("TaskSet OnStart")
	return nil
}

func (t *TaskSet) OnStop(ctx context.Context, u launce.User) error {
	if err := t.Random.OnStop(ctx, u); err != nil {
		return err
	}
	fmt.Println("TaskSet OnStop")
	return nil
}

func (t *TaskSet) task1(ctx context.Context, u launce.User, s taskset.Scheduler) error {
	fmt.Println("TaskSet task1")
	return nil
}

type SubTaskSet struct {
	*taskset.Sequential
}

func NewSubTaskSet() *SubTaskSet {
	t := &SubTaskSet{
		Sequential: taskset.NewSequential(
			taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				fmt.Println("SubTaskSet task1")
				return nil
			}),
			taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				fmt.Println("SubTaskSet task2")
				return nil
			}),
			taskset.TaskFunc(func(ctx context.Context, u launce.User, s taskset.Scheduler) error {
				fmt.Println("SubTaskSet quit")
				return taskset.InterruptTaskSet
			}),
		),
	}
	return t
}

func (t *SubTaskSet) OnStart(ctx context.Context, u launce.User, s taskset.Scheduler) error {
	fmt.Println("SubTaskSet OnStart")
	if err := t.Sequential.OnStart(ctx, u, s); err != nil {
		return err
	}
	return nil
}

func (t *SubTaskSet) OnStop(ctx context.Context, u launce.User) error {
	if err := t.Sequential.OnStop(ctx, u); err != nil {
		return err
	}
	fmt.Println("SubTaskSet OnStop")
	return nil
}
