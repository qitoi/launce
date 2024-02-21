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
	"errors"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/internal"
)

var (
	RescheduleTask              = errors.New("reschedule task")
	RescheduleTaskImmediately   = errors.New("reschedule task immediately")
	InterruptTaskSet            = errors.New("interrupt taskset")
	InterruptTaskSetImmediately = errors.New("interrupt taskset immediately")
)

type TaskSet interface {
	Task

	Len() int
	Next() Task
	WaitTime() launce.WaitTimeFunc
	OnStart(ctx context.Context, s Scheduler) error
	OnStop(ctx context.Context) error

	FilterTasks(f func(tasks []Task) []Task)
}

type taskQueue struct {
	tasks []Task
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
	return t
}

func (tq *taskQueue) Schedule(t Task, first bool) {
	if first {
		tq.tasks = append([]Task{t}, tq.tasks...)
	} else {
		tq.tasks = append(tq.tasks, t)
	}
}

func Run(ctx context.Context, t TaskSet, user launce.User) error {
	var tq taskQueue

	var waiter *internal.Waiter
	if waitTimeFunc := t.WaitTime(); waitTimeFunc != nil {
		waiter = &internal.Waiter{}
		waiter.Init(waitTimeFunc)
	}

	if err := t.OnStart(ctx, &tq); err != nil {
		if errors.Is(err, InterruptTaskSet) {
			return RescheduleTask
		} else if errors.Is(err, InterruptTaskSetImmediately) {
			return RescheduleTaskImmediately
		}
		return err
	}

	for {
		if tq.Empty() {
			tq.Schedule(t.Next(), false)
		}

		task := tq.Next()
		err := task.Run(ctx, user, &tq)

		switch {
		case err == nil || errors.Is(err, RescheduleTask):
			// next task with wait
			if err := wait(ctx, user, waiter); err != nil {
				return err
			}

		case errors.Is(err, RescheduleTaskImmediately):
			// next task without wait

		case errors.Is(err, launce.StopUser):
			_ = t.OnStop(ctx)
			return err

		case errors.Is(err, InterruptTaskSet):
			if err := t.OnStop(ctx); errors.Is(err, launce.StopUser) {
				return err
			}
			return RescheduleTask

		case errors.Is(err, InterruptTaskSetImmediately):
			if err := t.OnStop(ctx); errors.Is(err, launce.StopUser) {
				return err
			}
			return RescheduleTaskImmediately

		default:
			if user != nil {
				user.Runner().ReportException(err)
			}
			if err := wait(ctx, user, waiter); err != nil {
				return err
			}
		}
	}
}

func wait(ctx context.Context, user launce.User, waiter *internal.Waiter) error {
	// TaskSet に WaitTimeFunc が設定されていればそれを使用して Wait
	if waiter != nil {
		return waiter.Wait(ctx)
	}
	// 指定されていなければ User の Wait を使用
	return user.Wait(ctx)
}
