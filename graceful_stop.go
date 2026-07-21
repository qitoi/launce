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
	"time"
)

type stopSignalKey struct{}

// WithGracefulStop returns a context whose Done fires stopTimeout after ctx
// is done, instead of immediately (stopTimeout <= 0 cancels immediately).
// The original ctx is still queryable via Stopping. The returned cleanup
// must be called once the context is no longer needed.
func WithGracefulStop(ctx context.Context, stopTimeout time.Duration) (context.Context, func()) {
	gracefulCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	gracefulCtx = context.WithValue(gracefulCtx, stopSignalKey{}, ctx)

	stop := context.AfterFunc(ctx, func() {
		if stopTimeout > 0 {
			time.AfterFunc(stopTimeout, cancel)
		} else {
			cancel()
		}
	})

	return gracefulCtx, func() {
		stop()
		cancel()
	}
}

// Stopping reports whether ctx was derived via WithGracefulStop and the
// original context has already been asked to stop.
func Stopping(ctx context.Context) bool {
	soft, ok := ctx.Value(stopSignalKey{}).(context.Context)
	return ok && soft.Err() != nil
}

// WithoutGracefulStop returns a context with ctx's values, but that cancels
// as soon as the original context is done, skipping the grace period.
// Useful for operations with no in-flight work worth protecting, such as
// waiting between tasks. Returns ctx unchanged if it wasn't derived via
// WithGracefulStop.
func WithoutGracefulStop(ctx context.Context) context.Context {
	soft, ok := ctx.Value(stopSignalKey{}).(context.Context)
	if !ok {
		return ctx
	}
	return &withoutGracefulStopContext{Context: ctx, soft: soft}
}

// withoutGracefulStopContext uses ctx's Value/Deadline but soft's Done/Err.
type withoutGracefulStopContext struct {
	context.Context
	soft context.Context
}

func (s *withoutGracefulStopContext) Done() <-chan struct{} {
	return s.soft.Done()
}

func (s *withoutGracefulStopContext) Err() error {
	return s.soft.Err()
}
