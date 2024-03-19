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
	"errors"
	"math/rand"
	"net/http"
	"time"

	"github.com/qitoi/launce"
)

var (
	_ launce.BaseUser = (*User)(nil)
)

type User struct {
	launce.BaseUserImpl
}

func (u *User) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(1 * time.Second)
}

func (u *User) Process(ctx context.Context) error {
	s := time.Now()
	// do something
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
	responseTime := time.Now().Sub(s)
	contentLength := rand.Int63n(1024 * 1024)

	// report as successful
	u.Report(http.MethodGet, "/foo", responseTime, contentLength, nil)

	if err := u.Wait(ctx); err != nil {
		return err
	}

	s = time.Now()
	// do something
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
	responseTime = time.Now().Sub(s)
	contentLength = rand.Int63n(1024 * 1024)

	// report as failure
	u.Report(http.MethodGet, "/bar", responseTime, contentLength, errors.New("unexpected response status code"))

	if err := u.Wait(ctx); err != nil {
		return err
	}

	return nil
}
