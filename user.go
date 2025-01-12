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
	Report(requestType, name string, responseTime time.Duration, contentLength int64, err error)
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

// BaseUser is the interface that structs must implement to use BaseUserImpl.
type BaseUser interface {
	User
	WaitTime() WaitTimeFunc
}

// BaseUserImpl is a base implementation of User.
type BaseUserImpl struct {
	waiter   internal.Waiter
	runner   Runner
	reporter Reporter
}

// Init initializes the user.
func (b *BaseUserImpl) Init(u User, r Runner, rep Reporter) {
	if bu, ok := u.(BaseUser); !ok {
		panic("not implemented launce.BaseUser")
	} else {
		b.runner = r
		b.reporter = rep
		b.waiter.Init(bu.WaitTime())
	}
}

// Runner returns the generator.
func (b *BaseUserImpl) Runner() Runner {
	return b.runner
}

// Wait waits for the time returned by WaitTime.
func (b *BaseUserImpl) Wait(ctx context.Context) error {
	return b.waiter.Wait(ctx)
}

// OnStart is called when the user starts.
func (b *BaseUserImpl) OnStart(ctx context.Context) error {
	return nil
}

// OnStop is called when the user stops.
func (b *BaseUserImpl) OnStop(ctx context.Context) error {
	return nil
}

// Report reports the result of a request.
func (b *BaseUserImpl) Report(requestType, name string, responseTime time.Duration, contentLength int64, err error) {
	b.reporter.Report(requestType, name, responseTime, contentLength, err)
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
			if errors.Is(err, context.Canceled) || errors.Is(err, StopUser) {
				// locust/user stopped
				break
			}
			// unexpected error
			if r := user.Runner(); r != nil {
				// エラーのキャッチが指定されていない場合はエラーを返して終了
				if !r.CatchExceptions() {
					return err
				}

				// エラーをキャッチする場合はエラーをマスターに送信して継続
				r.ReportException(err)
			}
		}
	}

	return user.OnStop(context.WithoutCancel(ctx))
}
