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
	"sync/atomic"
	"time"

	"github.com/qitoi/launce/spawner"
	"github.com/qitoi/launce/stats"
)

const (
	defaultStatsNotifyInterval   = 100 * time.Millisecond
	defaultStatsAggregationUsers = 100
)

// UnknownUserError is an error that trying to spawn unregistered user.
type UnknownUserError struct {
	User string
}

func (e *UnknownUserError) Error() string {
	return fmt.Sprintf("unknown user %s", e.User)
}

// LoadGenerator is an instance of load test generator.
type LoadGenerator struct {
	// RestartMode is the mode of spawning users.
	RestartMode spawner.RestartMode

	// StatsNotifyInterval is the interval to notify stats.
	StatsNotifyInterval time.Duration

	// StatsAggregationUsers is the number of users per StatsAggregator
	StatsAggregationUsers int

	userSpawners map[string]*spawner.Spawner
	statsCh      chan *stats.Stats

	testStartHandlers    []func(ctx context.Context) error
	testStoppingHandlers []func(ctx context.Context)
	testStopHandlers     []func(ctx context.Context)

	// cancelStart is a function to cancel the OnTestStart handlers.
	cancelStart atomic.Value

	started atomic.Bool

	statsAggregator *stats.Aggregator
	aggMutex        sync.Mutex
	aggUserCounter  int
}

// NewLoadGenerator returns a new LoadGenerator.
func NewLoadGenerator() *LoadGenerator {
	return &LoadGenerator{
		RestartMode:           spawner.RestartNever,
		StatsNotifyInterval:   defaultStatsNotifyInterval,
		StatsAggregationUsers: defaultStatsAggregationUsers,

		userSpawners: map[string]*spawner.Spawner{},
		statsCh:      make(chan *stats.Stats),
	}
}

// RegisterUser registers a user class.
func (l *LoadGenerator) RegisterUser(r Runner, name string, f func() User) {
	spawnFunc := func(ctx context.Context) {
		l.aggMutex.Lock()
		if l.aggUserCounter == 0 {
			if l.statsAggregator != nil {
				l.statsAggregator.Release()
			}
			l.statsAggregator = stats.NewCollector(l.statsCh, l.StatsNotifyInterval)
			l.statsAggregator.Retain()
		}
		l.aggUserCounter = (l.aggUserCounter + 1) % l.StatsAggregationUsers
		rep := l.statsAggregator
		rep.Retain()
		l.aggMutex.Unlock()

		defer rep.Release()

		user := f()
		user.Init(user, r, rep)
		if err := processUser(ctx, user); err != nil {
			// ユーザー処理がエラーを返した場合はマスターに送信
			if !errors.Is(err, context.Canceled) {
				r.ReportException(err)
			}
		}
	}
	l.userSpawners[name] = spawner.New(spawnFunc, l.RestartMode)
}

// OnTestStart registers a function to be called when the load test starts.
func (l *LoadGenerator) OnTestStart(f func(ctx context.Context) error) {
	l.testStartHandlers = append(l.testStartHandlers, f)
}

// OnTestStopping registers a function to be called when the load test start stopping.
func (l *LoadGenerator) OnTestStopping(f func(ctx context.Context)) {
	l.testStoppingHandlers = append(l.testStoppingHandlers, f)
}

// OnTestStop registers a function to be called when the load test stops.
func (l *LoadGenerator) OnTestStop(f func(ctx context.Context)) {
	l.testStopHandlers = append(l.testStopHandlers, f)
}

// Start starts the load test.
func (l *LoadGenerator) Start() error {
	if started := l.started.Swap(true); started {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	l.cancelStart.Store(cancel)

	l.aggMutex.Lock()
	l.aggUserCounter = 0
	if l.statsAggregator != nil {
		l.statsAggregator.Release()
	}
	l.statsAggregator = nil
	l.aggMutex.Unlock()

	for _, f := range l.testStartHandlers {
		if err := f(ctx); err != nil {
			return err
		}
	}
	for _, s := range l.userSpawners {
		s.Cap(0)
		s.Start()
	}

	return nil
}

// Stop stops the load test.
func (l *LoadGenerator) Stop() {
	if started := l.started.Swap(false); !started {
		return
	}

	if f := l.cancelStart.Load(); f != nil {
		if cancel, ok := f.(context.CancelFunc); ok {
			cancel()
		}
	}

	ctx := context.Background()
	for _, f := range l.testStoppingHandlers {
		f(ctx)
	}

	var wg sync.WaitGroup
	wg.Add(len(l.userSpawners))
	for _, s := range l.userSpawners {
		go func(s *spawner.Spawner) {
			defer wg.Done()
			s.Stop()
			s.StopAllThreads()
		}(s)
	}
	wg.Wait()

	for _, f := range l.testStopHandlers {
		f(ctx)
	}
}

// Spawn spawns users.
func (l *LoadGenerator) Spawn(user string, count int) error {
	if s, ok := l.userSpawners[user]; ok {
		s.Cap(count)
		return nil
	}
	return &UnknownUserError{User: user}
}

// StopUsers stops all users.
func (l *LoadGenerator) StopUsers() {
	var wg sync.WaitGroup
	wg.Add(len(l.userSpawners))
	for _, s := range l.userSpawners {
		go func(s *spawner.Spawner) {
			defer wg.Done()
			s.Cap(0)
			s.StopAllThreads()
		}(s)
	}
	wg.Wait()
}

// Users returns the number of running users.
func (l *LoadGenerator) Users() map[string]int64 {
	ret := make(map[string]int64, len(l.userSpawners))
	for name, s := range l.userSpawners {
		ret[name] = s.Count()
	}
	return ret
}

// Stats returns the stats channel.
func (l *LoadGenerator) Stats() <-chan *stats.Stats {
	return l.statsCh
}
