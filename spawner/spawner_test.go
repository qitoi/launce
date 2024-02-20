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

package spawner_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/internal/mock"
	"github.com/qitoi/launce/spawner"
)

func Wait(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func NewSpawner(f func(ctx context.Context, u *mock.User) error) (*spawner.Spawner, *mock.UserStats, *mock.UserController) {
	gen, stats, uc := mock.UserGenerator(f)
	return spawner.New(func(ctx context.Context) {
		_ = launce.ProcessUser(ctx, gen())
	}, spawner.SpawnOnce), stats, uc
}

func TestSpawner_Start(t *testing.T) {
	s, stats, _ := NewSpawner(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	s.Start()
	defer func() {
		s.Stop()
		s.StopAllUsers()
	}()

	time.Sleep(100 * time.Millisecond)

	if n := stats.GetStarted(); n != 0 {
		t.Fatalf("unexpected task run. got:%d want:%d", n, 0)
	}
}

func TestSpawner_Cap_BeforeStart(t *testing.T) {
	s, stats, uc := NewSpawner(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	s.Cap(3)
	s.Start()

	uc.WaitStart(3)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Stop()
	s.StopAllUsers()

	uc.WaitStop(3)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 3 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}
	for id, c := range stats.GetLoop() {
		if c != 1 {
			t.Fatalf("unexpected task loop(%v). got:%d want:%d", id, c, 1)
		}
	}
}

func TestSpawner_Cap_AfterStart(t *testing.T) {
	s, stats, uc := NewSpawner(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	s.Start()
	s.Cap(3)

	uc.WaitStart(3)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Stop()
	s.StopAllUsers()

	uc.WaitStop(3)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 3 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}
	for id, c := range stats.GetLoop() {
		if c != 1 {
			t.Fatalf("unexpected task loop(%v). got:%d want:%d", id, c, 1)
		}
	}
}

func TestSpawner_Cap_Extension(t *testing.T) {
	s, stats, uc := NewSpawner(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	s.Cap(3)
	s.Start()

	uc.WaitStart(3)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Cap(5)

	uc.WaitStart(5)
	if n := stats.GetStarted(); n != 5 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 5)
	}
	if n := stats.GetStopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Stop()
	s.StopAllUsers()

	uc.WaitStop(5)
	if n := stats.GetStarted(); n != 5 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 5)
	}
	if n := stats.GetStopped(); n != 5 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 5)
	}
	for id, c := range stats.GetLoop() {
		if c != 1 {
			t.Fatalf("unexpected task loop(%v). got:%d want:%d", id, c, 1)
		}
	}
}

func TestSpawner_Cap_Shrink(t *testing.T) {
	s, stats, uc := NewSpawner(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	s.Cap(3)
	s.Start()

	uc.WaitStart(3)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Cap(1)

	uc.WaitStop(2)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}

	s.Stop()
	s.StopAllUsers()

	uc.WaitStop(3)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 3 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}
	for id, c := range stats.GetLoop() {
		if c != 1 {
			t.Fatalf("unexpected task loop(%v). got:%d want:%d", id, c, 1)
		}
	}
}

func TestSpawner_Cap_ShrinkMultiple(t *testing.T) {
	s, stats, uc := NewSpawner(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	s.Cap(6)
	s.Start()

	uc.WaitStart(6)
	if n := stats.GetStarted(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Cap(5)
	uc.WaitStop(1)
	if n := stats.GetStarted(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 6)
	}
	if n := stats.GetStopped(); n != 1 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 1)
	}

	s.Cap(4)
	uc.WaitStop(2)
	if n := stats.GetStarted(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 6)
	}
	if n := stats.GetStopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 2)
	}

	s.Cap(2)
	uc.WaitStop(2)
	if n := stats.GetStarted(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 6)
	}
	if n := stats.GetStopped(); n != 4 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 4)
	}

	s.Stop()
	s.StopAllUsers()

	uc.WaitStop(6)
	if n := stats.GetStarted(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 6)
	}
	if n := stats.GetStopped(); n != 6 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 6)
	}
	for id, c := range stats.GetLoop() {
		if c != 1 {
			t.Fatalf("unexpected task loop(%v). got:%d want:%d", id, c, 1)
		}
	}
}

func TestSpawner_Respawn_User(t *testing.T) {
	gen, stats, uc := mock.UserGenerator(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})
	s := spawner.New(func(ctx context.Context) {
		_ = launce.ProcessUser(ctx, gen())
	}, spawner.SpawnPersistent)

	s.Cap(1)
	s.Start()

	uc.WaitStart(1)
	if n := stats.GetStarted(); n != 1 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 1)
	}
	if n := stats.GetStopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.StopAllUsers()

	uc.WaitStart(2)
	if n := stats.GetStarted(); n != 2 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 2)
	}
	if n := stats.GetStopped(); n != 1 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 1)
	}

	s.Stop()
	s.StopAllUsers()

	uc.WaitStop(2)
	if n := stats.GetStarted(); n != 2 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 2)
	}
	if n := stats.GetStopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 2)
	}
	for id, c := range stats.GetLoop() {
		if c != 1 {
			t.Fatalf("unexpected task loop(%v). got:%d want:%d", id, c, 1)
		}
	}
}

func TestSpawner_TaskLoop(t *testing.T) {
	cond := sync.NewCond(&sync.Mutex{})
	signalCount := int64(0)

	s, stats, uc := NewSpawner(func(ctx context.Context, u *mock.User) error {
		userTaskCount := u.Value

		cond.L.Lock()
		defer cond.L.Unlock()

	loop:
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if signalCount > userTaskCount {
					break loop
				}
			}
			cond.Wait()
		}
		u.Value = signalCount

		return nil
	})

	s.Cap(3)
	s.Start()

	uc.WaitStart(3)

	// wait task loop: 1

	cond.L.Lock()
	signalCount += 1
	cond.L.Unlock()
	cond.Broadcast()

	// wait task loop: 2

	time.Sleep(100 * time.Millisecond)

	s.Stop()
	s.StopAllUsers()
	cond.Broadcast()

	// quit task loop: 2

	uc.WaitStop(3)
	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := stats.GetStopped(); n != 3 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}
	for id, c := range stats.GetLoop() {
		if c != 2 {
			t.Fatalf("unexpected task loop(%v). got:%d want:%d", id, c, 2)
		}
	}
}
