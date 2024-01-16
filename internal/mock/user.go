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

package mock

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/qitoi/launce"
)

type User struct {
	launce.BaseUser

	ID    int64
	Value int64

	task    func(ctx context.Context) error
	onStart func(ctx context.Context) error
	onStop  func(ctx context.Context) error
}

func (u *User) Process(ctx context.Context) error {
	return u.task(ctx)
}

func (u *User) OnStart(ctx context.Context) error {
	return u.onStart(ctx)
}

func (u *User) OnStop(ctx context.Context) error {
	return u.onStop(ctx)
}

type UserStats struct {
	mu      sync.Mutex
	Started int
	Stopped int
	Loop    map[int64]int
}

func (us *UserStats) GetStarted() int {
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.Started
}

func (us *UserStats) GetStopped() int {
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.Stopped
}

func (us *UserStats) GetLoop() map[int64]int {
	us.mu.Lock()
	defer us.mu.Unlock()
	m := map[int64]int{}
	for k, v := range us.Loop {
		m[k] = v
	}
	return m
}

func (us *UserStats) AddStarted() {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.Started += 1
}

func (us *UserStats) AddStopped() {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.Stopped += 1
}

func (us *UserStats) InitLoop(id int64) {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.Loop[id] = 0
}

func (us *UserStats) AddLoop(id int64) {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.Loop[id] += 1
}

type UserController struct {
	startCount uint64
	startCond  *sync.Cond
	stopCount  uint64
	stopCond   *sync.Cond
}

func (uc *UserController) Started() {
	uc.startCond.L.Lock()
	defer uc.startCond.L.Unlock()
	uc.startCount += 1
	uc.startCond.Broadcast()
}

func (uc *UserController) WaitStart(n uint64) {
	uc.startCond.L.Lock()
	for uc.startCount < n {
		uc.startCond.Wait()
	}
	uc.startCond.L.Unlock()
}

func (uc *UserController) Stopped() {
	uc.stopCond.L.Lock()
	defer uc.stopCond.L.Unlock()
	uc.stopCount += 1
	uc.stopCond.Broadcast()
}

func (uc *UserController) WaitStop(n uint64) {
	uc.stopCond.L.Lock()
	for uc.stopCount < n {
		uc.stopCond.Wait()
	}
	uc.stopCond.L.Unlock()
}

func UserGenerator(f func(ctx context.Context, u *User) error) (launce.UserGenerator, *UserStats, *UserController) {
	var id int64
	var stats UserStats
	stats.Loop = map[int64]int{}
	uc := &UserController{
		startCount: 0,
		startCond:  sync.NewCond(&sync.Mutex{}),
		stopCount:  0,
		stopCond:   sync.NewCond(&sync.Mutex{}),
	}

	generator := func() launce.User {
		uid := atomic.AddInt64(&id, 1)
		stats.InitLoop(uid)
		var u *User
		u = &User{
			ID: uid,
			onStart: func(ctx context.Context) error {
				stats.AddStarted()
				uc.Started()
				return nil
			},
			task: func(ctx context.Context) error {
				stats.AddLoop(uid)
				return f(ctx, u)
			},
			onStop: func(ctx context.Context) error {
				stats.AddStopped()
				uc.Stopped()
				return nil
			},
		}
		return u
	}

	return generator, &stats, uc
}
