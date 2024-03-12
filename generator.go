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
	"sync/atomic"
	"time"

	"github.com/qitoi/launce/spawner"
	"github.com/qitoi/launce/stats"
)

const (
	defaultStatsNotifyInterval = 100 * time.Millisecond
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

	userSpawners map[string]*spawner.Spawner
	statsCh      chan *stats.Stats

	testStartHandlers []func(ctx context.Context) error
	testStopHandlers  []func(ctx context.Context)

	// cancelStart is a function to cancel the OnTestStart handlers.
	cancelStart atomic.Value
}

// NewLoadGenerator returns a new LoadGenerator.
func NewLoadGenerator() *LoadGenerator {
	return &LoadGenerator{
		RestartMode:         spawner.RestartNever,
		StatsNotifyInterval: defaultStatsNotifyInterval,

		userSpawners: map[string]*spawner.Spawner{},
		statsCh:      make(chan *stats.Stats),
	}
}

// RegisterUser registers a user class.
func (l *LoadGenerator) RegisterUser(r Runner, name string, f func() User) {
	spawnFunc := func(ctx context.Context) {
		rep := &stats.Reporter{}
		rep.Start(l.statsCh, l.StatsNotifyInterval)
		defer rep.Stop()

		user := f()
		user.Init(user, r, rep)
		if err := processUser(ctx, user); err != nil {
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

// OnTestStop registers a function to be called when the load test stops.
func (l *LoadGenerator) OnTestStop(f func(ctx context.Context)) {
	l.testStopHandlers = append(l.testStopHandlers, f)
}

// Start starts the load test.
func (l *LoadGenerator) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	l.cancelStart.Store(cancel)

	for _, f := range l.testStartHandlers {
		if err := f(ctx); err != nil {
			return err
		}
	}
	for _, s := range l.userSpawners {
		s.Start()
	}

	return nil
}

// Stop stops the load test.
func (l *LoadGenerator) Stop() {
	if f := l.cancelStart.Load(); f != nil {
		if cancel, ok := f.(context.CancelFunc); ok {
			cancel()
		}
	}

	for _, s := range l.userSpawners {
		s.Stop()
		s.StopAllThreads()
	}
	ctx := context.Background()
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
	for _, s := range l.userSpawners {
		s.Cap(0)
		s.StopAllThreads()
	}
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
