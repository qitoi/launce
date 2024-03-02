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

var (
	_ Runner = (*LoadRunner)(nil)
)

// UnknownUserError is an error that trying to spawn unregistered user.
type UnknownUserError struct {
	User string
}

func (e *UnknownUserError) Error() string {
	return fmt.Sprintf("unknown user %s", e.User)
}

// MessageHandler defines a function to handle custom messages from the master.
type MessageHandler func(msg ReceivedMessage)

// Runner is the interface that runs the test.
type Runner interface {
	Host() string
	ParsedOptions() *ParsedOptions
	ReportException(err error)
	SendMessage(typ string, data any) error
}

// LoadRunner is an instance of load test runner.
type LoadRunner struct {
	// RestartMode is the mode of spawning users.
	RestartMode spawner.RestartMode

	// StatsNotifyInterval is the interval to notify stats.
	StatsNotifyInterval time.Duration

	// ReportExceptionFunc is a function to report an exception.
	ReportExceptionFunc func(error)

	// SendMessageFunc is a function to send a message to the master.
	SendMessageFunc func(typ string, data any) error

	userSpawners map[string]*spawner.Spawner
	statsCh      chan *stats.Stats

	host          atomic.Value
	parsedOptions atomic.Pointer[ParsedOptions]

	testStartHandlers []func(ctx context.Context) error
	testStopHandlers  []func(ctx context.Context)
	messageHandlers   map[string][]MessageHandler

	// cancelStart is a function to cancel the OnTestStart handlers.
	cancelStart atomic.Value
}

// NewLoadRunner returns a new LoadRunner.
func NewLoadRunner() *LoadRunner {
	return &LoadRunner{
		RestartMode:         spawner.RestartNever,
		StatsNotifyInterval: defaultStatsNotifyInterval,

		userSpawners:    map[string]*spawner.Spawner{},
		statsCh:         make(chan *stats.Stats),
		messageHandlers: map[string][]MessageHandler{},
	}
}

// Host returns the target host of the load test.
func (l *LoadRunner) Host() string {
	if h := l.host.Load(); l != nil {
		if s, ok := h.(string); ok {
			return s
		}
	}
	return ""
}

// SetHost sets the target host of the load test.
func (l *LoadRunner) SetHost(host string) {
	l.host.Store(host)
}

// ParsedOptions returns the parsed_options.
func (l *LoadRunner) ParsedOptions() *ParsedOptions {
	return l.parsedOptions.Load()
}

// SetParsedOptions sets the parsed_options.
func (l *LoadRunner) SetParsedOptions(options *ParsedOptions) {
	l.parsedOptions.Store(options)
}

// RegisterUser registers a user class.
func (l *LoadRunner) RegisterUser(name string, f func() User) {
	spawnFunc := func(ctx context.Context) {
		rep := &stats.Reporter{}
		rep.Start(l.statsCh, l.StatsNotifyInterval)
		defer rep.Stop()

		user := f()
		user.Init(user, l, rep)
		if err := processUser(ctx, user); err != nil {
			if !errors.Is(err, context.Canceled) {
				l.ReportException(err)
			}
		}
	}
	l.userSpawners[name] = spawner.New(spawnFunc, l.RestartMode)
}

// RegisterMessage registers a custom message handler.
func (l *LoadRunner) RegisterMessage(typ string, handler MessageHandler) {
	l.messageHandlers[typ] = append(l.messageHandlers[typ], handler)
}

// HandleMessage handles a custom message from the master.
func (l *LoadRunner) HandleMessage(msg ReceivedMessage) {
	if handlers, ok := l.messageHandlers[msg.Type]; ok {
		for _, handler := range handlers {
			handler(msg)
		}
	}
}

// SendMessage sends a message to the master.
func (l *LoadRunner) SendMessage(typ string, data any) error {
	if l.SendMessageFunc != nil {
		return l.SendMessageFunc(typ, data)
	}
	return nil
}

// OnTestStart registers a function to be called when the load test starts.
func (l *LoadRunner) OnTestStart(f func(ctx context.Context) error) {
	l.testStartHandlers = append(l.testStartHandlers, f)
}

// OnTestStop registers a function to be called when the load test stops.
func (l *LoadRunner) OnTestStop(f func(ctx context.Context)) {
	l.testStopHandlers = append(l.testStopHandlers, f)
}

// Start starts the load test.
func (l *LoadRunner) Start() error {
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
func (l *LoadRunner) Stop() {
	if f := l.cancelStart.Load(); f != nil {
		if cancel, ok := f.(context.CancelFunc); ok {
			cancel()
		}
	}

	for _, s := range l.userSpawners {
		s.Stop()
		s.StopAllUsers()
	}
	ctx := context.Background()
	for _, f := range l.testStopHandlers {
		f(ctx)
	}
}

// Spawn spawns users.
func (l *LoadRunner) Spawn(user string, count int) error {
	if s, ok := l.userSpawners[user]; ok {
		s.Cap(count)
		return nil
	}
	return &UnknownUserError{User: user}
}

// StopUsers stops all users.
func (l *LoadRunner) StopUsers() {
	for _, s := range l.userSpawners {
		s.Cap(0)
		s.StopAllUsers()
	}
}

// Users returns the number of running users.
func (l *LoadRunner) Users() map[string]int64 {
	ret := make(map[string]int64, len(l.userSpawners))
	for name, s := range l.userSpawners {
		ret[name] = s.Count()
	}
	return ret
}

// ReportException reports an exception.
func (l *LoadRunner) ReportException(err error) {
	if l.ReportExceptionFunc != nil {
		l.ReportExceptionFunc(err)
	}
}

// Stats returns the stats channel.
func (l *LoadRunner) Stats() <-chan *stats.Stats {
	return l.statsCh
}
