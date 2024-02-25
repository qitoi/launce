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

type UserRequirement interface {
	launce.BaseUserRequirement
	TaskSet() TaskSet
}

type User struct {
	launce.BaseUser
	taskset TaskSet
}

func (tu *User) Init(u launce.User, r launce.Runner, rep launce.Reporter) {
	if tur, ok := u.(UserRequirement); !ok {
		panic("not implemented taskset.UserRequirement")
	} else {
		tu.BaseUser.Init(u, r, rep)
		tu.taskset = tur.TaskSet()
	}
}

func (tu *User) OnStart(ctx context.Context) error {
	option := tu.Runner().ParsedOptions()

	var opts []filterOption
	if option.Tags != nil {
		opts = append(opts, includeTags(*option.Tags...))
	}
	if option.ExcludeTags != nil {
		opts = append(opts, excludeTags(*option.ExcludeTags...))
	}

	tu.taskset.FilterTasks(func(tasks []Task) []Task {
		return filterTasks(tasks, opts...)
	})

	return nil
}

func (tu *User) Process(ctx context.Context) error {
	err := Run(ctx, tu.taskset, tu)
	if err == nil || errors.Is(err, RescheduleTask) || errors.Is(err, RescheduleTaskImmediately) {
		return nil
	} else {
		return err
	}
}
