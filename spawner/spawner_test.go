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
	"sync/atomic"
	"testing"
	"time"

	"github.com/qitoi/launce/spawner"
)

type SpawnTask struct {
	started   int
	startCond *sync.Cond
	stopped   int
	stopCond  *sync.Cond
	TaskFunc  func(ctx context.Context)
}

func NewSpawnTask() *SpawnTask {
	return &SpawnTask{
		startCond: sync.NewCond(&sync.Mutex{}),
		stopCond:  sync.NewCond(&sync.Mutex{}),
	}
}

func (s *SpawnTask) Run(ctx context.Context) {
	s.startCond.L.Lock()
	s.started += 1
	s.startCond.Broadcast()
	s.startCond.L.Unlock()

	if s.TaskFunc != nil {
		s.TaskFunc(ctx)
	} else {
		<-ctx.Done()
	}

	s.stopCond.L.Lock()
	s.stopped += 1
	s.stopCond.Broadcast()
	s.stopCond.L.Unlock()
}

func (s *SpawnTask) Started() int {
	s.startCond.L.Lock()
	defer s.startCond.L.Unlock()
	return s.started
}

func (s *SpawnTask) Stopped() int {
	s.stopCond.L.Lock()
	defer s.stopCond.L.Unlock()
	return s.stopped
}

func (s *SpawnTask) WaitStart(n int) {
	s.startCond.L.Lock()
	for s.started < n {
		s.startCond.Wait()
	}
	s.startCond.L.Unlock()
}

func (s *SpawnTask) WaitStop(n int) {
	s.stopCond.L.Lock()
	for s.stopped < n {
		s.stopCond.Wait()
	}
	s.stopCond.L.Unlock()
}

func NewSpawner(mode spawner.RestartMode) (*spawner.Spawner, *SpawnTask) {
	st := NewSpawnTask()
	return spawner.New(st.Run, mode), st
}

func TestSpawner_Start(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	s.Start()
	defer func() {
		s.Stop()
		s.StopAllThreads()
	}()

	time.Sleep(100 * time.Millisecond)

	if n := st.Started(); n != 0 {
		t.Fatalf("unexpected task run. got:%d want:%d", n, 0)
	}
}

func TestSpawner_Cap_BeforeStart(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	s.Cap(3)
	s.Start()

	st.WaitStart(3)
	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Stop()
	s.StopAllThreads()

	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 3 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}
}

func TestSpawner_Cap_AfterStart(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	s.Start()
	s.Cap(3)

	st.WaitStart(3)
	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Stop()
	s.StopAllThreads()

	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 3 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}
}

func TestSpawner_Cap_Extension(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	s.Cap(3)
	s.Start()

	st.WaitStart(3)
	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Cap(5)

	st.WaitStart(5)
	if n := st.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 5)
	}
	if n := st.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Stop()
	s.StopAllThreads()

	if n := st.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 5)
	}
	if n := st.Stopped(); n != 5 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 5)
	}
}

func TestSpawner_Cap_Shrink(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	s.Cap(3)
	s.Start()

	st.WaitStart(3)
	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Cap(1)

	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}

	s.Stop()
	s.StopAllThreads()

	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 3 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 3)
	}
}

func TestSpawner_Cap_ShrinkMultiple(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	s.Cap(6)
	s.Start()

	st.WaitStart(6)
	if n := st.Started(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.Cap(5)
	if n := st.Started(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 6)
	}
	if n := st.Stopped(); n != 1 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 1)
	}

	s.Cap(4)
	if n := st.Started(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 6)
	}
	if n := st.Stopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 2)
	}

	s.Cap(2)
	if n := st.Started(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 6)
	}
	if n := st.Stopped(); n != 4 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 4)
	}

	s.Stop()
	s.StopAllThreads()

	if n := st.Started(); n != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 6)
	}
	if n := st.Stopped(); n != 6 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 6)
	}
}

func TestSpawner_SpawnPersistent(t *testing.T) {
	s, st := NewSpawner(spawner.RestartAlways)

	s.Cap(1)
	s.Start()

	st.WaitStart(1)
	if n := st.Started(); n != 1 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 1)
	}
	if n := st.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	s.StopAllThreads()

	if n := st.Stopped(); n != 1 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 1)
	}

	st.WaitStart(2)
	if n := st.Started(); n != 2 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 2)
	}
	if n := st.Stopped(); n != 1 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 1)
	}

	s.Stop()
	s.StopAllThreads()

	if n := st.Started(); n != 2 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 2)
	}
	if n := st.Stopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 2)
	}
}

func TestSpawner_StopAllThreads(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	ctx2, cancel := context.WithCancel(context.Background())

	var no atomic.Int64
	st.TaskFunc = func(ctx context.Context) {
		if no.Add(1) == 1 {
			<-ctx.Done()
		} else {
			<-ctx2.Done()
		}
	}

	s.Start()

	s.Cap(2)

	st.WaitStart(2)
	if n := st.Started(); n != 2 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 2)
	}
	if n := st.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 0)
	}

	var stopped atomic.Bool
	ch := make(chan struct{})
	go func() {
		s.StopAllThreads()
		stopped.Store(true)
		close(ch)
	}()

	st.WaitStop(1)

	time.Sleep(100 * time.Millisecond)

	if n := st.Started(); n != 2 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 2)
	}
	if n := st.Stopped(); n != 1 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 1)
	}
	if stopped.Load() {
		t.Fatalf("unexpected stopped all threads")
	}

	cancel()
	<-ch

	if n := st.Started(); n != 2 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 2)
	}
	if n := st.Stopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 2)
	}

	s.Stop()
}
