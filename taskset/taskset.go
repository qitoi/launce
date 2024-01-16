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
	OnStart(ctx context.Context) error
	OnStop(ctx context.Context) error

	ApplyFilter(opts ...FilterOption)
}

func Run(ctx context.Context, t TaskSet, user launce.User) error {
	waiter := launce.Waiter{}
	waiter.Init(t.WaitTime())

	if err := t.OnStart(ctx); err != nil {
		if errors.Is(err, InterruptTaskSet) {
			return RescheduleTask
		} else if errors.Is(err, InterruptTaskSetImmediately) {
			return RescheduleTaskImmediately
		}
		return err
	}

	for {
		task := t.Next()
		err := task.Run(ctx, user)

		switch {
		case err == nil || errors.Is(err, RescheduleTask):
			if err := wait(ctx, user, waiter); err != nil {
				return err
			}
			break

		case errors.Is(err, RescheduleTaskImmediately):
			break

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
				user.ReportExceptions(err)
			}
			if err := wait(ctx, user, waiter); err != nil {
				return err
			}
		}
	}
}

type FilterOptions struct {
	tags        *[]string
	excludeTags []string
}

type FilterOption func(opt *FilterOptions)

func IncludeTags(tags ...string) FilterOption {
	return func(opt *FilterOptions) {
		opt.tags = &tags
	}
}

func ExcludeTags(tags ...string) FilterOption {
	return func(opt *FilterOptions) {
		opt.excludeTags = tags
	}
}

func FilterTasks(tasks []Task, opts ...FilterOption) []Task {
	var option FilterOptions
	for _, opt := range opts {
		opt(&option)
	}

	var tagMap map[string]struct{}
	if option.tags != nil {
		tagMap = make(map[string]struct{}, len(*option.tags))
		for _, tag := range *option.tags {
			tagMap[tag] = struct{}{}
		}
	}

	excludeTags := option.excludeTags
	excludeTagMap := make(map[string]struct{})
	for _, tag := range excludeTags {
		excludeTagMap[tag] = struct{}{}
	}

	filtered := make([]Task, 0, len(tasks))
loop:
	for _, task := range tasks {
		taskTags := GetTags(task)

		for _, tag := range taskTags {
			if _, ok := excludeTagMap[tag]; ok {
				continue loop
			}
		}

		add := false

		if option.tags != nil {
			for _, tag := range taskTags {
				if _, ok := tagMap[tag]; ok {
					add = true
					break
				}
			}
		} else {
			add = true
		}

		if ts := unwrapTaskSet(task); ts != nil {
			if add {
				ts.ApplyFilter(ExcludeTags(option.excludeTags...))
			} else {
				ts.ApplyFilter(opts...)
			}
			add = ts.Len() > 0
		}

		if add {
			filtered = append(filtered, task)
		}
	}

	return filtered
}

func unwrapTaskSet(task Task) TaskSet {
	for task != nil {
		if taskset, ok := task.(TaskSet); ok {
			return taskset
		} else if t, ok := task.(interface{ Unwrap() Task }); ok {
			task = unwrapTaskSet(t.Unwrap())
		} else {
			task = nil
		}
	}
	return nil
}

func wait(ctx context.Context, user launce.User, waiter launce.Waiter) error {
	if user != nil {
		if err := user.Wait(ctx); err == nil {
			return nil
		} else if !errors.Is(err, launce.ErrWaitFuncUndefined) {
			return err
		}
	}
	return waiter.Wait(ctx)
}
