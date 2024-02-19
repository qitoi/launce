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
	"slices"
	"sync"
	"sync/atomic"
)

type SpawnMode int

const (
	SpawnOnce SpawnMode = iota
	SpawnPersistent
)

type SpawnFunc func(ctx context.Context)

type Spawner struct {
	mode      SpawnMode
	spawnFunc SpawnFunc
	count     int

	spawnChMutex sync.Mutex
	spawnCh      chan struct{}

	stop func()

	users userList

	spawned int64
}

func NewSpawner(f SpawnFunc, mode SpawnMode) *Spawner {
	return &Spawner{
		mode:      mode,
		spawnFunc: f,
	}
}

func (s *Spawner) Cap(count int) {
	if s.count == count {
		return
	}

	s.count = count

	if s.isRunning() {
		// spawnWorker が動いている場合は一度止めて別の spawnWorkerを動かす
		s.Start()
	} else {
		// spawnWorker が動いていない場合はユーザーリストの上限のみ変更
		s.users.Cap(count)
	}
}

func (s *Spawner) Count() int64 {
	return s.spawned
}

func (s *Spawner) Start() {
	s.Stop()
	s.spawnWorker(s.count)
}

func (s *Spawner) Stop() {
	if s.stop != nil {
		s.stop()
	}
}

func (s *Spawner) StopAllUsers() {
	s.users.Clear()
}

func (s *Spawner) isRunning() bool {
	return s.stop != nil
}

func (s *Spawner) spawnWorker(count int) {
	spawnCh := make(chan struct{}, count)
	stopCh := make(chan struct{})
	stopWaitCh := make(chan struct{})
	s.stop = func() {
		// spawn用の goroutine に停止の指令を出して、 goroutine が終了するのを待つ
		close(stopCh)
		<-stopWaitCh
		s.stop = nil
	}

	s.spawnChMutex.Lock()
	oldSpawnCh := s.spawnCh
	s.spawnCh = spawnCh
	s.spawnChMutex.Unlock()

	// 旧 spawnCh から新 spawnCh にユーザーのロックを移し、溢れた場合はユーザーを捨てる
	s.migrateSpawnCh(oldSpawnCh, spawnCh)
	s.users.Cap(count)

	if s.mode == SpawnOnce {
		go s.spawnWorkerOnce(spawnCh, stopCh, stopWaitCh)
	} else if s.mode == SpawnPersistent {
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
	id := s.users.Add(cancel)

	go func() {
		atomic.AddInt64(&s.spawned, 1)
		defer atomic.AddInt64(&s.spawned, -1)
		s.spawnFunc(ctx)
		s.users.Delete(id)

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
			s.users.Pop(1)
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
			s.users.Pop(1)
			dst <- v
		}
	}
}

type userWorker struct {
	id     uint64
	cancel func()
}

type userList struct {
	mu        sync.Mutex
	count     int
	users     []userWorker
	currentID uint64
}

func (ul *userList) Cap(n int) {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.count = n
	ul.pop(len(ul.users) - n)
	old := ul.users
	ul.users = make([]userWorker, len(old), n)
	copy(ul.users, old)
}

func (ul *userList) Add(cancel func()) uint64 {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.currentID += 1
	ul.users = append(ul.users, userWorker{
		id:     ul.currentID,
		cancel: cancel,
	})
	ul.pop(len(ul.users) - ul.count)
	return ul.currentID
}

func (ul *userList) Pop(n int) {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.pop(n)
}

func (ul *userList) Delete(id uint64) {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.users = slices.DeleteFunc(ul.users, func(u userWorker) bool {
		if u.id == id {
			u.cancel()
			return true
		}
		return false
	})
}

func (ul *userList) Clear() {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.pop(len(ul.users))
}

func (ul *userList) pop(n int) {
	n = min(n, len(ul.users))
	if n <= 0 {
		return
	}
	deleted := ul.users[:n]
	ul.users = ul.users[n:]
	for _, u := range deleted {
		u.cancel()
	}
}
