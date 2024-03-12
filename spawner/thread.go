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

package spawner

import (
	"slices"
	"sync"
)

type thread struct {
	id     uint64
	cancel func()
}

type threadList struct {
	mu        sync.Mutex
	count     int
	threads   []thread
	currentID uint64
}

func (tl *threadList) Cap(n int) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.count = n
	tl.pop(len(tl.threads) - n)
	old := tl.threads
	tl.threads = make([]thread, len(old), n)
	copy(tl.threads, old)
}

func (tl *threadList) Add(cancel func()) uint64 {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.currentID += 1
	tl.threads = append(tl.threads, thread{
		id:     tl.currentID,
		cancel: cancel,
	})
	tl.pop(len(tl.threads) - tl.count)
	return tl.currentID
}

func (tl *threadList) Pop(n int) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.pop(n)
}

func (tl *threadList) Delete(id uint64) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.threads = slices.DeleteFunc(tl.threads, func(u thread) bool {
		if u.id == id {
			u.cancel()
			return true
		}
		return false
	})
}

func (tl *threadList) Clear() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.pop(len(tl.threads))
}

func (tl *threadList) pop(n int) {
	n = min(n, len(tl.threads))
	if n <= 0 {
		return
	}
	deleted := tl.threads[:n]
	tl.threads = tl.threads[n:]
	for _, u := range deleted {
		u.cancel()
	}
}
