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
	// RestartNever is a mode where goroutines are not restarted once they finish.
	RestartNever RestartMode = iota
	// RestartAlways is a mode that ensures goroutines are automatically restarted to maintain the capacity.
	RestartAlways
)

// Spawner is a goroutine manager.
type Spawner struct {
	mode      RestartMode
	spawnFunc SpawnFunc
	count     int64
	spawned   int64

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
		s.spawnWorker(count)
	} else {
		// spawnWorker が動いていない場合はユーザーリストの上限のみ変更
		s.threads.Cap(count)
	}
}

// Count returns the number of currently running goroutines.
func (s *Spawner) Count() int64 {
	return s.spawned
}

// Start starts the goroutine manager.
func (s *Spawner) Start() {
	s.spawnWorker(int(atomic.LoadInt64(&s.count)))
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

func (s *Spawner) spawnWorker(count int) {
	spawnCh := make(chan struct{}, count)
	stopCh := make(chan struct{})
	stopWaitCh := make(chan struct{})

	s.stopMutex.Lock()
	if s.stop != nil {
		s.stop()
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
	s.threads.Cap(count)

	if s.mode == RestartNever {
		go s.spawnWorkerOnce(spawnCh, stopCh, stopWaitCh)
	} else if s.mode == RestartAlways {
		go s.spawnWorkerPersistent(spawnCh, stopCh, stopWaitCh)
	}
}

func (s *Spawner) spawnWorkerOnce(spawnCh, stopCh, stopWaitCh chan struct{}) {
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
			s.threads.Pop(1)
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
			s.threads.Pop(1)
			dst <- v
		}
	}
}
