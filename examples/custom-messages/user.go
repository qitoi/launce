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
	"time"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/taskset"
)

type User struct {
	taskset.User
}

func (u *User) Init(r launce.Runner, waitTime launce.WaitTimeFunc) {
	u.User.Init(r, waitTime)
	u.SetTaskSet(taskset.NewSequential(
		taskset.TaskFunc(func(ctx context.Context, user launce.User) error {
			return nil
		}),
	))
}

func (u *User) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(1 * time.Second)
}