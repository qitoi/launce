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

	"github.com/qitoi/launce/internal"
)

var (
	// StopUser is an error that stops the user.
	StopUser = errors.New("stop user")
)

const (
	// NoneResponseTime is a special value that represents no response time.
	NoneResponseTime = time.Duration(-1)
)

// Reporter is the interface that reports the result of a request.
type Reporter interface {
	Report(requestType, name string, responseTime time.Duration, contentLength int64)
	ReportError(requestType, name string, responseTime time.Duration, contentLength int64, err error)
}

// User is the interface that behaves as a locust user.
type User interface {
	Reporter

	Init(u User, r Runner, rep Reporter)
	Runner() Runner
	OnStart(ctx context.Context) error
	OnStop(ctx context.Context) error
	Process(ctx context.Context) error
	Wait(ctx context.Context) error
}

// BaseUserRequirement is the interface that structs must implement to use BaseUser.
type BaseUserRequirement interface {
	User
	WaitTime() WaitTimeFunc
}

// BaseUser is a base implementation of User.
type BaseUser struct {
	waiter   internal.Waiter
	runner   Runner
	reporter Reporter
}

// Init initializes the user.
func (b *BaseUser) Init(u User, r Runner, rep Reporter) {
	if bu, ok := u.(BaseUserRequirement); !ok {
		panic("not implemented launce.BaseUserRequirement")
	} else {
		b.runner = r
		b.reporter = rep
		b.waiter.Init(bu.WaitTime())
	}
}

// Runner returns the generator.
func (b *BaseUser) Runner() Runner {
	return b.runner
}

// Wait waits for the time returned by WaitTime.
func (b *BaseUser) Wait(ctx context.Context) error {
	return b.waiter.Wait(ctx)
}

// OnStart is called when the user starts.
func (b *BaseUser) OnStart(ctx context.Context) error {
	return nil
}

// OnStop is called when the user stops.
func (b *BaseUser) OnStop(ctx context.Context) error {
	return nil
}

// Report reports the result of a request.
func (b *BaseUser) Report(requestType, name string, responseTime time.Duration, contentLength int64) {
	b.reporter.Report(requestType, name, responseTime, contentLength)
}

// ReportError reports the result of a request with an error.
func (b *BaseUser) ReportError(requestType, name string, responseTime time.Duration, contentLength int64, err error) {
	b.reporter.ReportError(requestType, name, responseTime, contentLength, err)
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
		if ctx.Err() != nil {
			// user.Process がコンテキストのキャンセルを無視した場合 goroutine が停止しなくなるのでここでもチェックする
			break
		}
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
