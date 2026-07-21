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
	"net/http"
	"sync/atomic"
	"time"

	"github.com/qitoi/launce"
)

// taskDuration はタスク1回あたりの所要時間。--stop-timeout をこれより
// 長く指定すると、停止要求が来てもタスクが最後まで完了するようになる。
const taskDuration = 3 * time.Second

var (
	_ launce.BaseUser = (*User)(nil)

	idCounter atomic.Int64
)

type User struct {
	launce.BaseUserImpl
	id int64
}

func (u *User) OnStart(ctx context.Context) error {
	u.id = idCounter.Add(1)
	return nil
}

func (u *User) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(1 * time.Second)
}

func (u *User) Process(ctx context.Context) error {
	fmt.Printf("[%s] id=%d task 開始\n", now(), u.id)

	start := time.Now()
	const step = 200 * time.Millisecond
	for elapsed := time.Duration(0); elapsed < taskDuration; elapsed += step {
		select {
		case <-ctx.Done():
			fmt.Printf("[%s] id=%d task 中断 (経過 %s)\n", now(), u.id, time.Since(start).Round(step))
			return ctx.Err()
		case <-time.After(step):
		}
	}

	responseTime := time.Since(start)
	fmt.Printf("[%s] id=%d task 完了 (経過 %s)\n", now(), u.id, responseTime.Round(step))
	u.Report(http.MethodGet, "/task", responseTime, 0, nil)

	return u.Wait(ctx)
}

func now() string {
	return time.Now().Format("15:04:05.000")
}
