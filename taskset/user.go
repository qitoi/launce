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

// UserRequirement is an interface that structs must implement to use User.
type UserRequirement interface {
	launce.BaseUserRequirement

	// TaskSet returns the taskset to be processed.
	TaskSet() TaskSet
}

// User processes taskset.
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
	tags, exTags := tu.Runner().Tags()

	var opts []filterOption
	if tags != nil {
		opts = append(opts, includeTags(*tags...))
	}
	if exTags != nil {
		opts = append(opts, excludeTags(*exTags...))
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
