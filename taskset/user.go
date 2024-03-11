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

// User is an interface that structs must implement to use UserImpl.
type User interface {
	launce.BaseUser

	// TaskSet returns the taskset to be processed.
	TaskSet() TaskSet
}

// UserImpl processes taskset.
type UserImpl struct {
	launce.BaseUserImpl
	taskset TaskSet
}

func (tu *UserImpl) Init(u launce.User, r launce.Runner, rep launce.Reporter) {
	if tur, ok := u.(User); !ok {
		panic("not implemented taskset.User")
	} else {
		tu.BaseUserImpl.Init(u, r, rep)
		tu.taskset = tur.TaskSet()
	}
}

func (tu *UserImpl) OnStart(ctx context.Context) error {
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

func (tu *UserImpl) Process(ctx context.Context) error {
	err := run(ctx, tu.taskset, tu)
	if err == nil || errors.Is(err, RescheduleTask) || errors.Is(err, RescheduleTaskImmediately) {
		return nil
	} else {
		return err
	}
}
