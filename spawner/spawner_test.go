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
	s, st := NewSpawner(spawner.RestartLocustCompatible)

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
	s, st := NewSpawner(spawner.RestartLocustCompatible)

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
	s, st := NewSpawner(spawner.RestartLocustCompatible)

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
	s, st := NewSpawner(spawner.RestartLocustCompatible)

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
	s, st := NewSpawner(spawner.RestartLocustCompatible)

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
	s, st := NewSpawner(spawner.RestartLocustCompatible)

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

func TestSpawner_RestartNever_Cap_AfterStart(t *testing.T) {
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

// TestSpawner_RestartNever_Cap_Shrink は、Cap の縮小で強制キャンセルされたスレッドは、
// target を再度増やした際に累計 spawn 数から差し引かれている分だけ
// 新規に spawn され直すことを確認する。
func TestSpawner_RestartNever_Cap_Shrink(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	s.Cap(3)
	s.Start()

	st.WaitStart(3)
	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}

	s.Cap(1)
	st.WaitStop(2)

	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}
	if n := st.Stopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 2)
	}

	// target を縮小前の 3 に戻すと、強制キャンセルされた 2 スロット分の
	// 累計 spawn 数が戻っているため、新規に 2 件 spawn される
	s.Cap(3)

	st.WaitStart(5)
	if n := st.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 5)
	}

	s.Stop()
	s.StopAllThreads()

	if n := st.Stopped(); n != 5 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", n, 5)
	}
}

// TestSpawner_RestartNever_ShrinkRegrow_KeepsFailedSlotsUnfilled は、
// OnStart 失敗のように自ら終了したスレッドのスロットは再拡大しても埋め直されない一方、
// Cap の縮小で強制キャンセルされたスレッドのスロットは再拡大時に埋め直されることを
// 同時に確認する。
func TestSpawner_RestartNever_ShrinkRegrow_KeepsFailedSlotsUnfilled(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	// 最初に完了した 2 件は OnStart 失敗を模して即終了、残りはブロックし続ける
	var n atomic.Int64
	st.TaskFunc = func(ctx context.Context) {
		if n.Add(1) <= 2 {
			return
		}
		<-ctx.Done()
	}

	s.Start()
	s.Cap(4)

	st.WaitStart(4)
	st.WaitStop(2)

	if got := st.Started(); got != 4 {
		t.Fatalf("unexpected started user. got:%d want:%d", got, 4)
	}
	if got := st.Stopped(); got != 2 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", got, 2)
	}

	// 残り2件が生存したまま target を 4 に保っても、自然終了した 2 件分は埋め直されない
	time.Sleep(100 * time.Millisecond)
	if got := st.Started(); got != 4 {
		t.Fatalf("unexpected started user after settle. got:%d want:%d", got, 4)
	}

	// 生存中の残り 2 件を Cap(0) で強制キャンセルする
	s.Cap(0)
	st.WaitStop(4)
	if got := st.Stopped(); got != 4 {
		t.Fatalf("unexpected stopped user. got:%d want:%d", got, 4)
	}

	// 縮小前の target 4 まで再拡大すると、強制キャンセルされた 2 件分のみ
	// 新規に spawn される (自然終了 (失敗) した 2 件分は復活しない)
	s.Cap(4)

	st.WaitStart(6)
	if got := st.Started(); got != 6 {
		t.Fatalf("unexpected started user. got:%d want:%d", got, 6)
	}

	s.Stop()
	s.StopAllThreads()
}

// TestSpawner_RestartNever_NoBackfill は、一度 spawn したスロットが失敗などで即終了した場合でも、
// target を増やした際に累計 spawn 数を基準に不足分のみを spawn し、
// 終了済みのスロットを埋め直さないことを確認する。
func TestSpawner_RestartNever_NoBackfill(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)

	// OnStart が即座に失敗して終了するケースを模倣し、タスクを即終了させる
	st.TaskFunc = func(ctx context.Context) {}

	s.Start()
	s.Cap(3)

	st.WaitStart(3)
	st.WaitStop(3)

	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}

	// target を 3 から 5 に増やしても、現在生きているユーザーは 0 人だが、
	// 累計 spawn 数 (3) を基準に不足分 (5-3=2) だけが新規に spawn される。
	// 現在生きている数 (0) を基準にすると 5 件 spawn されてしまい、started は 8 になるはず。
	s.Cap(5)

	st.WaitStart(5)
	st.WaitStop(5)

	if n := st.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 5)
	}

	s.Stop()
	s.StopAllThreads()
}

// TestSpawner_RestartNever_Start_ResetsTotalSpawned は、Start() の呼び出しで
// 累計 spawn 数がリセットされ、新しいテスト実行では改めて target 分まで
// spawn できることを確認する。
func TestSpawner_RestartNever_Start_ResetsTotalSpawned(t *testing.T) {
	s, st := NewSpawner(spawner.RestartNever)
	st.TaskFunc = func(ctx context.Context) {}

	s.Start()
	s.Cap(3)

	st.WaitStart(3)
	st.WaitStop(3)

	s.Stop()
	s.StopAllThreads()

	if n := st.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 3)
	}

	// 新しいテスト実行を模倣 (LoadGenerator.Start() と同様に Cap(0) してから Start())
	s.Cap(0)
	s.Start()
	s.Cap(2)

	// リセットされていなければ budget は 2-3<0 で 0 件になり、started は 3 のまま変わらない
	st.WaitStart(5)
	if n := st.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%d want:%d", n, 5)
	}

	s.Stop()
	s.StopAllThreads()
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
	s, st := NewSpawner(spawner.RestartLocustCompatible)

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
