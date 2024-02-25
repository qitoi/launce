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

package internal

import (
	"context"
	"time"
)

type Waiter struct {
	waitTime func() time.Duration
	timer    *time.Timer
}

func (w *Waiter) Init(waitTime func() time.Duration) {
	w.waitTime = waitTime
	w.timer = time.NewTimer(0)
	if !w.timer.Stop() {
		<-w.timer.C
	}
}

func (w *Waiter) Wait(ctx context.Context) error {
	if w.waitTime == nil {
		return ctx.Err()
	}

	d := w.waitTime()
	w.timer.Reset(d)

	select {
	case <-w.timer.C:
		return nil

	case <-ctx.Done():
		if !w.timer.Stop() {
			<-w.timer.C
		}
		return ctx.Err()
	}
}
