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
	"errors"
	"fmt"
	"sync"
	"time"
)

type Runner struct {
	SpawnMode         SpawnMode
	ExceptionReporter func(error)

	userSpawners map[string]*Spawner
	statistics   *Statistics

	parsedOptions      *ParsedOptions
	parsedOptionsMutex sync.RWMutex
}

func NewRunner() (*Runner, error) {
	return &Runner{
		SpawnMode: SpawnOnce,

		userSpawners: map[string]*Spawner{},
		statistics:   newStatistics(),
	}, nil
}

func (r *Runner) SetParsedOptions(options *ParsedOptions) {
	r.parsedOptionsMutex.Lock()
	defer r.parsedOptionsMutex.Unlock()
	r.parsedOptions = options
}

func (r *Runner) RegisterUser(name string, f UserGenerator) {
	spawnFunc := func(ctx context.Context) {
		r.parsedOptionsMutex.RLock()
		parsedOptions := r.parsedOptions
		r.parsedOptionsMutex.RUnlock()

		ctx = withParsedOptions(ctx, parsedOptions)

		user := f()
		user.Init(r, user.WaitTime())
		if err := ProcessUser(ctx, user); err != nil {
			if !errors.Is(err, context.Canceled) {
			}
		}
	}
	r.userSpawners[name] = NewSpawner(spawnFunc, r.SpawnMode)
}

func (r *Runner) Start() {
	for _, spawner := range r.userSpawners {
		spawner.Start()
	}
}

func (r *Runner) Stop() {
	for _, spawner := range r.userSpawners {
		spawner.Stop()
		spawner.StopAllUsers()
	}
}

func (r *Runner) Spawn(user string, count int) error {
	if spawner, ok := r.userSpawners[user]; ok {
		spawner.Cap(count)
		return nil
	}
	return fmt.Errorf("unknown user spawn: %v, %v", user, count)
}

func (r *Runner) StopUsers() {
	for _, spawner := range r.userSpawners {
		spawner.Cap(0)
		spawner.StopAllUsers()
	}
}

func (r *Runner) Users() map[string]int64 {
	ret := make(map[string]int64, len(r.userSpawners))
	for name, spawner := range r.userSpawners {
		ret[name] = spawner.Count()
	}
	return ret
}

func (r *Runner) Report(requestType, name string, opts ...StatisticsOption) {
	r.statistics.Add(time.Now(), requestType, name, opts...)
}

func (r *Runner) ReportException(err error) {
	if r.ExceptionReporter != nil {
		r.ExceptionReporter(err)
	}
}

func (r *Runner) FlushStats() (StatisticsEntries, *StatisticsEntry, StatisticsErrors) {
	return r.statistics.Move()
}
