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
	_ launce.User = (*User)(nil)
)

type User struct {
	launce.BaseUser
	taskset TaskSet
}

func (u *User) OnStart(ctx context.Context) error {
	option := u.Runner().ParsedOptions()

	var opts []FilterOption
	if option.Tags != nil {
		opts = append(opts, IncludeTags(*option.Tags...))
	}
	if option.ExcludeTags != nil {
		opts = append(opts, ExcludeTags(*option.ExcludeTags...))
	}

	u.taskset.ApplyFilter(opts...)

	return nil
}

func (u *User) OnStop(ctx context.Context) error {
	return nil
}

func (u *User) SetTaskSet(taskset TaskSet) {
	u.taskset = taskset
}

func (u *User) Process(ctx context.Context) error {
	err := u.taskset.Run(ctx, u)
	if err == nil || errors.Is(err, RescheduleTask) || errors.Is(err, RescheduleTaskImmediately) {
		return nil
	} else {
		return err
	}
}
