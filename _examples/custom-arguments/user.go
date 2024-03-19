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
)

var (
	_ launce.BaseUser = (*User)(nil)
)

type args struct {
	Str string `msgpack:"my_arg_str"`
}

type User struct {
	launce.BaseUserImpl
	param string
}

func (u *User) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(1 * time.Second)
}

func (u *User) OnStart(ctx context.Context) error {
	var v args
	if err := u.Runner().Options(&v); err != nil {
		return err
	}
	u.param = v.Str
	return nil
}

func (u *User) Process(ctx context.Context) error {
	fmt.Printf("user task: %v\n", u.param)
	return u.Wait(ctx)
}
