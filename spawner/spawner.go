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

// Package spawner implements dynamic goroutine management.
package spawner

import (
	"context"
	"sync"
	"sync/atomic"
)

// RestartMode is a mode for spawning threads.
type RestartMode int

// SpawnFunc is a target function for concurrent execution.
type SpawnFunc func(ctx context.Context)

const (
	// RestartLocustCompatible backfills a finished goroutine's slot on the next Cap call,
	// matching the original Locust worker; goroutines finishing early (e.g. a failing
	// OnStart) can make spawns per Cap call snowball. Use RestartNever to avoid that.
	RestartLocustCompatible RestartMode = iota
	// RestartNever never backfills a slot once a goroutine finishes on its own, unlike
	// RestartLocustCompatible. A slot freed by Cap reducing the capacity is still given
	// back, so a later Cap increase can spawn up to the new capacity.
	RestartNever
	// RestartAlways is a mode that ensures goroutines are automatically restarted to maintain the capacity.
	RestartAlways
)

// Spawner is a goroutine manager.
type Spawner struct {
	mode      RestartMode
	spawnFunc SpawnFunc
	count     int64
	spawned   int64

	totalSpawned int64

	spawnChMutex sync.Mutex
	spawnCh      chan struct{}

	stop      func()
	stopMutex sync.Mutex

	threads     threadList
	threadMutex sync.Mutex
	threadWg    *sync.WaitGroup
}

// New returns a new Spawner.
func New(f SpawnFunc, mode RestartMode) *Spawner {
	s := &Spawner{
		mode:      mode,
		spawnFunc: f,
	}
	s.threadWg = &sync.WaitGroup{}
	return s
}

// Cap sets the maximum number of concurrent goroutines.
func (s *Spawner) Cap(count int) {
	if atomic.SwapInt64(&s.count, int64(count)) == int64(count) {
		return
	}

	if s.isRunning() {
		// spawnWorker が動いている場合は一度止めて別の spawnWorkerを動かす
		s.spawnWorker(count, false)
	} else {
		// spawnWorker が動いていない場合はユーザーリストの上限のみ変更
		s.reclaim(s.threads.Cap(count))
	}
}

func (s *Spawner) reclaim(canceled int) {
	if canceled > 0 {
		atomic.AddInt64(&s.totalSpawned, -int64(canceled))
	}
}

// Count returns the number of currently running goroutines.
func (s *Spawner) Count() int64 {
	return s.spawned
}

// Start starts the goroutine manager.
func (s *Spawner) Start() {
	s.spawnWorker(int(atomic.LoadInt64(&s.count)), true)
}

// Stop stops the goroutine manager.
func (s *Spawner) Stop() {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	if s.stop != nil {
		s.stop()
		s.stop = nil
	}
}

// StopAllThreads stops all running goroutines.
func (s *Spawner) StopAllThreads() {
	s.threadMutex.Lock()
	wg := s.threadWg
	s.threadWg = &sync.WaitGroup{}
	s.threads.Clear()
	s.threadMutex.Unlock()

	wg.Wait()
}

func (s *Spawner) isRunning() bool {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	return s.stop != nil
}

func (s *Spawner) spawnWorker(count int, reset bool) {
	spawnCh := make(chan struct{}, count)
	stopCh := make(chan struct{})
	stopWaitCh := make(chan struct{})

	s.stopMutex.Lock()
	if s.stop != nil {
		s.stop()
	}
	if reset {
		// 前回の spawn goroutine が完全に停止したのを確認したあとで
		// 累計 spawn 数をリセットする
		atomic.StoreInt64(&s.totalSpawned, 0)
	}
	s.stop = func() {
		// spawn用の goroutine に停止の指令を出して、 goroutine が終了するのを待つ
		close(stopCh)
		<-stopWaitCh
	}
	s.stopMutex.Unlock()

	s.spawnChMutex.Lock()
	oldSpawnCh := s.spawnCh
	s.spawnCh = spawnCh
	s.spawnChMutex.Unlock()

	// 旧 spawnCh から新 spawnCh にユーザーのロックを移し、溢れた場合はユーザーを捨てる
	s.migrateSpawnCh(oldSpawnCh, spawnCh)
	s.reclaim(s.threads.Cap(count))

	switch s.mode {
	case RestartLocustCompatible:
		go s.spawnWorkerLocustCompatible(spawnCh, stopCh, stopWaitCh)
	case RestartNever:
		// 累計 spawn 数が count に満たない分だけ新規に spawn する
		budget := max(0, count-int(atomic.LoadInt64(&s.totalSpawned)))
		go s.spawnWorkerOnce(spawnCh, stopCh, stopWaitCh, budget)
	case RestartAlways:
		go s.spawnWorkerPersistent(spawnCh, stopCh, stopWaitCh)
	}
}

func (s *Spawner) spawnWorkerLocustCompatible(spawnCh, stopCh, stopWaitCh chan struct{}) {
	defer close(stopWaitCh)
	defer close(spawnCh)

	for {
		select {
		case spawnCh <- struct{}{}:
			// spawnCh のバッファに空きがある場合はユーザーを新規に spawn させる
			s.spawn()

		case <-stopCh:
			return

		default:
			// 指定数を起動したので終了
			return
		}
	}
}

func (s *Spawner) spawnWorkerOnce(spawnCh, stopCh, stopWaitCh chan struct{}, budget int) {
	defer close(stopWaitCh)
	defer close(spawnCh)

	for n := 0; n < budget; n++ {
		select {
		case spawnCh <- struct{}{}:
			// budget の範囲内でのみユーザーを新規に spawn させる
			s.spawn()

		case <-stopCh:
			return
		}
	}
}

func (s *Spawner) spawnWorkerPersistent(spawnCh, stopCh, stopWaitCh chan struct{}) {
	defer close(stopWaitCh)
	defer close(spawnCh)

	for {
		select {
		case spawnCh <- struct{}{}:
			// spawnCh のバッファに空きがある場合はユーザーを新規に spawn させる
			s.spawn()

		case <-stopCh:
			return
		}
	}
}

func (s *Spawner) spawn() {
	atomic.AddInt64(&s.totalSpawned, 1)

	ctx, cancel := context.WithCancel(context.Background())

	s.threadMutex.Lock()
	id := s.threads.Add(cancel)
	wg := s.threadWg
	wg.Add(1)
	s.threadMutex.Unlock()

	go func() {
		defer wg.Done()

		atomic.AddInt64(&s.spawned, 1)
		defer atomic.AddInt64(&s.spawned, -1)

		s.spawnFunc(ctx)
		s.threads.Delete(id)

		// ユーザーが終了したので spawn のロックを 1 つ解除
		s.spawnChMutex.Lock()
		<-s.spawnCh
		s.spawnChMutex.Unlock()
	}()
}

func (s *Spawner) migrateSpawnCh(src, dst chan struct{}) {
	if src == nil {
		return
	}

	if cap(dst) == 0 {
		// cap が 0 に更新された場合は動いているユーザーを全て捨てる
		for v := range src {
			s.reclaim(s.threads.Pop(1))
			dst <- v
		}
		return
	}

loop:
	for {
		var v struct{}
		var ok bool

		select {
		case v, ok = <-src:
			// 移行元の channel から spawn のロックを取り出せなくなったら終了
			if !ok {
				break loop
			}
		}

		select {
		case dst <- v:
			// 移行先の channel に spawn のロックを移せられれば次のロックを取り出す
		default:
			// 移行先の channel が一杯の場合はユーザーを1つ捨ててからロックを移す
			s.reclaim(s.threads.Pop(1))
			dst <- v
		}
	}
}
