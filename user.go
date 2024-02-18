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

package launce

import (
	"context"
	"errors"
	"time"
)

var (
	StopUser = errors.New("stop user")
)

var (
	ErrWaitFuncUndefined = errors.New("wait func undefined")
)

func Wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		d = 0
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type User interface {
	Init(r Runner, waitTime WaitTimeFunc)

	Runner() Runner

	Wait(ctx context.Context) error
	WaitTime() WaitTimeFunc

	OnStart(ctx context.Context) error
	OnStop(ctx context.Context) error
	Process(ctx context.Context) error
}

type BaseUser struct {
	Waiter
	runner Runner
}

func (b *BaseUser) Init(r Runner, waitTime WaitTimeFunc) {
	b.runner = r
	b.Waiter.Init(waitTime)
}

func (b *BaseUser) Runner() Runner {
	return b.runner
}

func (b *BaseUser) WaitTime() WaitTimeFunc {
	return Constant(0)
}

func (b *BaseUser) OnStart(ctx context.Context) error {
	return nil
}

func (b *BaseUser) OnStop(ctx context.Context) error {
	return nil
}

func ProcessUser(ctx context.Context, user User) error {
	if err := user.OnStart(ctx); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, StopUser) {
			// unexpected error
			return err
		}
		// locust/user stopped
		return user.OnStop(context.WithoutCancel(ctx))
	}

	for {
		if err := user.Process(ctx); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, StopUser) {
				// unexpected error
				return err
			}
			// locust/user stopped
			break
		}
	}

	return user.OnStop(context.WithoutCancel(ctx))
}
