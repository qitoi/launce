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

	"github.com/qitoi/launce/internal"
	"github.com/qitoi/launce/stats"
)

var (
	StopUser = errors.New("stop user")
)

type Reporter interface {
	Report(requestType, name string, opts ...stats.Option)
}

type User interface {
	Init(u User, r Runner, rep Reporter)
	Runner() Runner

	Report(requestType, name string, opts ...stats.Option)

	Wait(ctx context.Context) error

	OnStart(ctx context.Context) error
	OnStop(ctx context.Context) error
	Process(ctx context.Context) error
}

type BaseUserRequirement interface {
	User
	WaitTime() WaitTimeFunc
}

type BaseUser struct {
	waiter   internal.Waiter
	runner   Runner
	reporter Reporter
}

func (b *BaseUser) Init(u User, r Runner, rep Reporter) {
	if bu, ok := u.(BaseUserRequirement); !ok {
		panic("not implemented launce.BaseUserRequirement")
	} else {
		b.runner = r
		b.reporter = rep
		b.waiter.Init(bu.WaitTime())
	}
}

func (b *BaseUser) Runner() Runner {
	return b.runner
}

func (b *BaseUser) Report(requestType, name string, opts ...stats.Option) {
	b.reporter.Report(requestType, name, opts...)
}

func (b *BaseUser) Wait(ctx context.Context) error {
	return b.waiter.Wait(ctx)
}

func (b *BaseUser) OnStart(ctx context.Context) error {
	return nil
}

func (b *BaseUser) OnStop(ctx context.Context) error {
	return nil
}

func processUser(ctx context.Context, user User) error {
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
