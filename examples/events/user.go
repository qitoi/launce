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
	"time"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/taskset"
)

var (
	_ taskset.User = (*User)(nil)
)

type User struct {
	taskset.UserImpl
	ID int64
}

func (u *User) TaskSet() taskset.TaskSet {
	return NewTaskSet()
}

func (u *User) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(1 * time.Second)
}

func (u *User) OnStart(ctx context.Context) error {
	if err := u.BaseUserImpl.OnStart(ctx); err != nil {
		return err
	}
	fmt.Printf("User #%d OnStart\n", u.ID)
	return nil
}

func (u *User) OnStop(ctx context.Context) error {
	if err := u.BaseUserImpl.OnStop(ctx); err != nil {
		return err
	}
	fmt.Printf("User #%d OnStop\n", u.ID)
	return nil
}
