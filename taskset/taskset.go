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

// Package taskset implements utilities for structured test scenarios.
package taskset

import (
	"context"
	"errors"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/internal"
)

var (
	// RescheduleTask is an error to reschedule task, equivalent to return nil.
	RescheduleTask = errors.New("reschedule task")
	// RescheduleTaskImmediately is an error to reschedule task without waiting.
	RescheduleTaskImmediately = errors.New("reschedule task immediately")
	// InterruptTaskSet is an error to exit from the current TaskSet.
	InterruptTaskSet = errors.New("interrupt taskset")
	// InterruptTaskSetImmediately  is an error to exit from the current TaskSet without waiting.
	InterruptTaskSetImmediately = errors.New("interrupt taskset immediately")
)

// TaskSet represents a set of tasks used in structured test scenarios.
type TaskSet interface {
	Task

	Len() int
	Next() Task
	WaitTime() launce.WaitTimeFunc
	OnStart(ctx context.Context, u launce.User, s Scheduler) error
	OnStop(ctx context.Context, u launce.User) error

	FilterTasks(f func(tasks []Task) []Task)
}

type BaseImpl struct {
	taskset TaskSet
}

func (b *BaseImpl) Init(task Task) {
	if ts := unwrapTask[TaskSet](task); ts != nil {
		b.taskset = ts
	}
}

func (b *BaseImpl) Run(ctx context.Context, user launce.User, s Scheduler) error {
	var tq taskQueue

	var waiter *internal.Waiter
	if waitTimeFunc := b.taskset.WaitTime(); waitTimeFunc != nil {
		waiter = &internal.Waiter{}
		waiter.Init(waitTimeFunc)
	}

	if err := b.taskset.OnStart(ctx, user, &tq); err != nil {
		if errors.Is(err, InterruptTaskSet) {
			return RescheduleTask
		} else if errors.Is(err, InterruptTaskSetImmediately) {
			return RescheduleTaskImmediately
		}
		return err
	} else if err := ctx.Err(); err != nil {
		return err
	}

	for {
		if tq.Empty() {
			tq.Schedule(b.taskset.Next(), false)
		}

		task := tq.Next()
		if ti := unwrapTask[interface{ Init(task Task) }](task); ti != nil {
			ti.Init(task)
		}
		err := task.Run(ctx, user, &tq)

		switch {
		case err == nil || errors.Is(err, RescheduleTask):
			// next task with wait

		case errors.Is(err, RescheduleTaskImmediately):
			// next task without wait
			continue

		case errors.Is(err, launce.StopUser):
			// user stopped by task
			_ = b.taskset.OnStop(ctx, user)
			return err

		case errors.Is(err, context.Canceled):
			// test stopped
			_ = b.taskset.OnStop(context.WithoutCancel(ctx), user)
			return err

		case errors.Is(err, InterruptTaskSet):
			// stop current taskset
			if err := b.taskset.OnStop(ctx, user); errors.Is(err, launce.StopUser) || errors.Is(err, context.Canceled) {
				return err
			}
			return RescheduleTask

		case errors.Is(err, InterruptTaskSetImmediately):
			// stop current taskset without parent taskset wait
			if err := b.taskset.OnStop(ctx, user); errors.Is(err, launce.StopUser) || errors.Is(err, context.Canceled) {
				return err
			}
			return RescheduleTaskImmediately

		default:
			if user != nil {
				if r := user.Runner(); r != nil {
					// エラーのキャッチが指定されていない場合はエラーを返して終了
					if !r.CatchExceptions() {
						return err
					}

					// エラーをキャッチする場合はエラーをマスターに送信して継続
					r.ReportException(err)
				}
			}
		}

		if err := wait(ctx, user, waiter); err != nil {
			if errors.Is(err, context.Canceled) {
				_ = b.taskset.OnStop(context.WithoutCancel(ctx), user)
			}
			return err
		}
	}
}

func (b *BaseImpl) WaitTime() launce.WaitTimeFunc {
	return nil
}

func (b *BaseImpl) OnStart(ctx context.Context, u launce.User, s Scheduler) error {
	return nil
}

func (b *BaseImpl) OnStop(ctx context.Context, u launce.User) error {
	return nil
}

func wait(ctx context.Context, user launce.User, waiter *internal.Waiter) error {
	// TaskSet に WaitTimeFunc が設定されていればそれを使用して Wait
	if waiter != nil {
		return waiter.Wait(ctx)
	}
	// 指定されていなければ User の Wait を使用
	if user != nil {
		return user.Wait(ctx)
	}
	return ctx.Err()
}

func run(ctx context.Context, task Task, user launce.User) error {
	if ti, ok := task.(interface{ Init(task Task) }); ok {
		ti.Init(task)
	}
	return task.Run(ctx, user, nil)
}
