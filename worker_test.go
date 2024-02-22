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

package launce_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/qitoi/launce"
)

var (
	parsedOptions = launce.ParsedOptions{}
)

func TestWorker_Join(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	wg.Add(1)
	go func() {
		defer wg.Done()
		waitForReady()
		w.Quit()
	}()

	if err := w.Join(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	wg.Wait()
}

func TestWorker_Quit(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	waitForReady()

	w.Quit()

	wg.Wait()
}

func TestWorker_RegisterMessage(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	ch := make(chan launce.ReceivedMessage)
	w.RegisterMessage("custom-message", func(msg launce.ReceivedMessage) {
		ch <- msg
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	waitForReady()

	_ = master.Send(launce.Message{
		Type:   "custom-message",
		Data:   "hello",
		NodeID: w.ClientID,
	})

	customMessage := <-ch

	var msg string
	if err := customMessage.DecodePayload(&msg); err != nil {
		t.Fatalf("payload decode error: %v", err)
	}

	if customMessage.Type != "custom-message" {
		t.Fatalf("received custome message type mismatch. got:%v want:%v", customMessage.Type, "custom-message")
	}
	if customMessage.NodeID != w.ClientID {
		t.Fatalf("received custome message node_id mismatch. got:%v want:%v", customMessage.NodeID, w.ClientID)
	}
	if msg != "hello" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg, "hello")
	}

	w.Quit()

	wg.Wait()
}

func TestWorker_RegisterMessage_MultipleReceivers(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	ch := make(chan string)
	w.RegisterMessage("custom-message", func(msg launce.ReceivedMessage) {
		var s string
		_ = msg.DecodePayload(&s)
		ch <- "1:" + s
	})
	w.RegisterMessage("custom-message", func(msg launce.ReceivedMessage) {
		var s string
		_ = msg.DecodePayload(&s)
		ch <- "2:" + s
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	waitForReady()

	_ = master.Send(launce.Message{
		Type:   "custom-message",
		Data:   "hello",
		NodeID: w.ClientID,
	})

	msg1 := <-ch
	if msg1 != "1:hello" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg1, "1:hello")
	}
	msg2 := <-ch
	if msg2 != "2:hello" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg2, "2:hello")
	}

	w.Quit()

	wg.Wait()
}

func TestWorker_RegisterMessage_MultipleMessages(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	ch := make(chan string, 1)
	w.RegisterMessage("custom-message1", func(msg launce.ReceivedMessage) {
		var s string
		_ = msg.DecodePayload(&s)
		ch <- "1:" + s
	})
	w.RegisterMessage("custom-message2", func(msg launce.ReceivedMessage) {
		var s string
		_ = msg.DecodePayload(&s)
		ch <- "2:" + s
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	waitForReady()

	_ = master.Send(launce.Message{
		Type:   "custom-message1",
		Data:   "foo",
		NodeID: w.ClientID,
	})
	_ = master.Send(launce.Message{
		Type:   "custom-message2",
		Data:   "bar",
		NodeID: w.ClientID,
	})

	msg1 := <-ch
	if msg1 != "1:foo" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg1, "1:foo")
	}
	msg2 := <-ch
	if msg2 != "2:bar" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg2, "2:bar")
	}

	w.Quit()

	wg.Wait()
}

func TestWorker_RegisterUser(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	uc := newUserController()

	w.RegisterUser("test-user", uc.NewUser)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	waitForReady()

	_ = master.SendSpawn(map[string]int64{
		"test-user": 3,
	}, w.ClientID)

	uc.WaitStart(3)

	if n := uc.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 3)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 5,
	}, w.ClientID)

	uc.WaitStart(5)

	if n := uc.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 1,
	}, w.ClientID)

	uc.WaitStop(4)

	if n := uc.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}
	if n := uc.Stopped(); n != 4 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 4)
	}

	w.Quit()
	uc.WaitStop(5)

	if n := uc.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}
	if n := uc.Stopped(); n != 5 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 5)
	}

	wg.Wait()
}

func TestWorker_SpawnMessage(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	masterCh, _ := startMasterReceiver(&wg, master, launce.MessageClientReady, launce.MessageSpawning, launce.MessageSpawningComplete, launce.MessageClientStopped)

	uc := newUserController()

	w.RegisterUser("test-user", uc.NewUser)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	msg := <-masterCh
	if msg.Type != launce.MessageClientReady {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageClientReady)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 3,
	}, w.ClientID)

	msg = <-masterCh
	if msg.Type != launce.MessageSpawning {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageSpawning)
	}

	msg = <-masterCh
	if msg.Type != launce.MessageSpawningComplete {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageSpawningComplete)
	}

	uc.WaitStart(3)

	_ = master.Send(launce.Message{
		Type:   launce.MessageStop,
		Data:   nil,
		NodeID: w.ClientID,
	})

	msg = <-masterCh
	if msg.Type != launce.MessageClientStopped {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageClientStopped)
	}

	uc.WaitStop(3)

	msg = <-masterCh
	if msg.Type != launce.MessageClientReady {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageClientReady)
	}

	w.Quit()

	wg.Wait()
}

func TestWorker_QuitMessage(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	masterCh, waitForReady := startMasterReceiver(&wg, master, launce.MessageClientReady, launce.MessageStats, launce.MessageQuit)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	waitForReady()

	if msg := <-masterCh; msg.Type != launce.MessageClientReady {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageClientReady)
	}

	_ = master.Send(launce.Message{
		Type:   launce.MessageQuit,
		Data:   nil,
		NodeID: w.ClientID,
	})

	if msg := <-masterCh; msg.Type != launce.MessageStats {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageStats)
	}

	if msg := <-masterCh; msg.Type != launce.MessageQuit {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageQuit)
	}

	wg.Wait()
}

func setupWorker(t *testing.T) (*launce.Worker, *masterTransport) {
	t.Helper()

	mt, wt, err := makeTransportSet()
	if err != nil {
		t.Fatal(err)
	}

	w, err := launce.NewWorker(wt)
	if err != nil {
		t.Fatal(err)
	}

	w.HeartbeatInterval = 0
	w.StatsReportInterval = 0
	w.MetricsMonitorInterval = 0

	return w, &masterTransport{transport: mt}
}

func startMasterReceiver(wg *sync.WaitGroup, master *masterTransport, filterMessages ...string) (<-chan launce.ReceivedMessage, func()) {
	ch := make(chan launce.ReceivedMessage)

	readyCh := make(chan struct{})
	var once sync.Once

	wg.Add(1)
	go func() {
		defer close(ch)
		defer wg.Done()

		for {
			msg, err := master.Receive()
			if err != nil {
				return
			}
			if msg.Type == "client_ready" {
				_ = master.Send(launce.Message{
					Type:   launce.MessageAck,
					Data:   launce.AckPayload{Index: 1},
					NodeID: msg.NodeID,
				})
				once.Do(func() {
					close(readyCh)
				})
			}
			if slices.Contains(filterMessages, msg.Type) {
				ch <- msg
			}
		}
	}()

	return ch, func() {
		<-readyCh
	}
}

var (
	_ launce.BaseUserRequirement = (*user)(nil)
)

type user struct {
	launce.BaseUser
	ProcessFunc func(ctx context.Context) error
	OnStartFunc func(ctx context.Context) error
	OnStopFunc  func(ctx context.Context) error
}

func (u *user) Process(ctx context.Context) error {
	return u.ProcessFunc(ctx)
}

func (u *user) OnStart(ctx context.Context) error {
	return u.OnStartFunc(ctx)
}

func (u *user) OnStop(ctx context.Context) error {
	return u.OnStopFunc(ctx)
}

func (u *user) WaitTime() launce.WaitTimeFunc {
	return nil
}

type userController struct {
	started   int
	startCond *sync.Cond
	stopped   int
	stopCond  *sync.Cond
}

func newUserController() *userController {
	return &userController{
		startCond: sync.NewCond(&sync.Mutex{}),
		stopCond:  sync.NewCond(&sync.Mutex{}),
	}
}

func (s *userController) NewUser() launce.User {
	return &user{
		ProcessFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		OnStartFunc: func(ctx context.Context) error {
			s.OnStart()
			return nil
		},
		OnStopFunc: func(ctx context.Context) error {
			s.OnStop()
			return nil
		},
	}
}

func (s *userController) OnStart() {
	s.startCond.L.Lock()
	s.started += 1
	s.startCond.Broadcast()
	s.startCond.L.Unlock()
}

func (s *userController) OnStop() {
	s.stopCond.L.Lock()
	s.stopped += 1
	s.stopCond.Broadcast()
	s.stopCond.L.Unlock()
}

func (s *userController) Started() int {
	s.startCond.L.Lock()
	defer s.startCond.L.Unlock()
	return s.started
}

func (s *userController) Stopped() int {
	s.stopCond.L.Lock()
	defer s.stopCond.L.Unlock()
	return s.stopped
}

func (s *userController) WaitStart(n int) {
	s.startCond.L.Lock()
	for s.started < n {
		s.startCond.Wait()
	}
	s.startCond.L.Unlock()
}

func (s *userController) WaitStop(n int) {
	s.stopCond.L.Lock()
	for s.stopped < n {
		s.stopCond.Wait()
	}
	s.stopCond.L.Unlock()
}

type masterTransport struct {
	transport *transport
	lastSpawn float64
}

func (m *masterTransport) Send(msg launce.Message) error {
	b, err := launce.EncodeMessage(msg)
	if err != nil {
		return err
	}
	return m.transport.Send(b)
}

func (m *masterTransport) Receive() (launce.ReceivedMessage, error) {
	b, err := m.transport.Receive()
	if err != nil {
		return launce.ReceivedMessage{}, err
	}
	msg, err := launce.DecodeMessage(b)
	if err != nil {
		return launce.ReceivedMessage{}, err
	}
	return msg, nil
}

func (m *masterTransport) SendSpawn(users map[string]int64, nodeID string) error {
	m.lastSpawn += 1
	return m.Send(launce.Message{
		Type: launce.MessageSpawn,
		Data: launce.SpawnPayload{
			Timestamp:        m.lastSpawn,
			UserClassesCount: users,
			Host:             "",
			StopTimeout:      0,
			ParsedOptions:    parsedOptions,
		},
		NodeID: nodeID,
	})
}

var (
	_ launce.Transport = (*transport)(nil)
)

var (
	ErrConnectionClosed = errors.New("connection closed")
)

type transport struct {
	Dest   *transport
	sendCh chan<- []byte
	recvCh <-chan []byte
	ctx    context.Context
	cancel func()
	mu     sync.RWMutex
	wg     sync.WaitGroup
}

func (m *transport) Open(ctx context.Context, clientID string) error {
	if m == m.Dest.Dest && m.sendCh != nil && m.Dest.recvCh != nil {
		return nil
	}
	ch1 := make(chan []byte, 100)
	ch2 := make(chan []byte, 100)
	m.sendCh, m.Dest.recvCh = ch1, ch1
	m.recvCh, m.Dest.sendCh = ch2, ch2
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.Dest.ctx, m.Dest.cancel = context.WithCancel(context.Background())
	return nil
}

func (m *transport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendCh == nil {
		return nil
	}

	m.cancel()
	m.wg.Wait()
	close(m.sendCh)
	m.sendCh = nil
	m.recvCh = nil

	return nil
}

func (m *transport) Send(msg []byte) error {
	m.mu.RLock()
	ch := m.sendCh
	m.mu.RUnlock()

	m.wg.Add(1)
	defer m.wg.Done()

	select {
	case ch <- msg:
		return nil

	case <-m.ctx.Done():
		return ErrConnectionClosed
	}
}

func (m *transport) Receive() ([]byte, error) {
	m.mu.RLock()
	ch := m.recvCh
	m.mu.RUnlock()

	select {
	case b, ok := <-ch:
		if !ok {
			_ = m.Close()
			return nil, ErrConnectionClosed
		}
		return b, nil

	case <-m.ctx.Done():
		return nil, ErrConnectionClosed
	}
}

func makeTransportSet() (*transport, *transport, error) {
	t1 := &transport{}
	t2 := &transport{}
	t1.Dest, t2.Dest = t2, t1
	if err := t2.Open(context.Background(), ""); err != nil {
		return nil, nil, err
	}
	return t1, t2, nil
}
