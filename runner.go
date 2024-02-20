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
)

var (
	_ Runner = (*LoadRunner)(nil)
)

type MessageHandler func(msg ReceivedMessage)

type Runner interface {
	Host() string
	ParsedOptions() *ParsedOptions
	Report(requestType, name string, opts ...StatisticsOption)
	ReportException(err error)
	SendMessage(typ string, data any) error
}

type LoadRunner struct {
	SpawnMode           spawner.SpawnMode
	ReportExceptionFunc func(error)
	SendMessageFunc     func(typ string, data any) error

	userSpawners map[string]*spawner.Spawner
	statistics   *Statistics

	host          atomic.Value
	parsedOptions atomic.Pointer[ParsedOptions]

	testStartHandlers []func(ctx context.Context) error
	testStopHandlers  []func(ctx context.Context)
	messageHandlers   map[string][]MessageHandler

	cancelStart atomic.Value
}

func New() (*LoadRunner, error) {
	return &LoadRunner{
		SpawnMode: spawner.SpawnOnce,

		userSpawners: map[string]*spawner.Spawner{},
		statistics:   NewStatistics(),

		messageHandlers: map[string][]MessageHandler{},
	}, nil
}

func (l *LoadRunner) Host() string {
	if h := l.host.Load(); l != nil {
		if s, ok := h.(string); ok {
			return s
		}
	}
	return ""
}

func (l *LoadRunner) SetHost(host string) {
	l.host.Store(host)
}

func (l *LoadRunner) ParsedOptions() *ParsedOptions {
	return l.parsedOptions.Load()
}

func (l *LoadRunner) SetParsedOptions(options *ParsedOptions) {
	l.parsedOptions.Store(options)
}

func (l *LoadRunner) RegisterUser(name string, f func() User) {
	spawnFunc := func(ctx context.Context) {
		user := f()
		user.Init(l, user.WaitTime())
		if err := ProcessUser(ctx, user); err != nil {
			if !errors.Is(err, context.Canceled) {
			}
		}
	}
	l.userSpawners[name] = spawner.New(spawnFunc, l.SpawnMode)
}

func (l *LoadRunner) RegisterMessage(typ string, handler MessageHandler) {
	l.messageHandlers[typ] = append(l.messageHandlers[typ], handler)
}

func (l *LoadRunner) HandleMessage(msg ReceivedMessage) {
	if handlers, ok := l.messageHandlers[msg.Type]; ok {
		for _, handler := range handlers {
			handler(msg)
		}
	}
}

func (l *LoadRunner) SendMessage(typ string, data any) error {
	if l.SendMessageFunc != nil {
		return l.SendMessageFunc(typ, data)
	}
	return nil
}

func (l *LoadRunner) OnTestStart(f func(ctx context.Context) error) {
	l.testStartHandlers = append(l.testStartHandlers, f)
}

func (l *LoadRunner) OnTestStop(f func(ctx context.Context)) {
	l.testStopHandlers = append(l.testStopHandlers, f)
}

func (l *LoadRunner) Start() error {
	l.FlushStats()

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

func (l *LoadRunner) Spawn(user string, count int) error {
	if s, ok := l.userSpawners[user]; ok {
		s.Cap(count)
		return nil
	}
	return fmt.Errorf("unknown user spawn: %v, %v", user, count)
}

func (l *LoadRunner) StopUsers() {
	for _, s := range l.userSpawners {
		s.Cap(0)
		s.StopAllUsers()
	}
}

func (l *LoadRunner) Users() map[string]int64 {
	ret := make(map[string]int64, len(l.userSpawners))
	for name, s := range l.userSpawners {
		ret[name] = s.Count()
	}
	return ret
}

func (l *LoadRunner) Report(requestType, name string, opts ...StatisticsOption) {
	l.statistics.Add(time.Now(), requestType, name, opts...)
}

func (l *LoadRunner) ReportException(err error) {
	if l.ReportExceptionFunc != nil {
		l.ReportExceptionFunc(err)
	}
}

func (l *LoadRunner) FlushStats() (StatisticsEntries, *StatisticsEntry, StatisticsErrors) {
	return l.statistics.Move()
}
