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
	"sync/atomic"
	"testing"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/internal/mock"
	"github.com/qitoi/launce/internal/worker"
)

var (
	parsedOptions, _ = msgpack.Marshal(launce.ParsedOptions{})
)

type MasterTransport struct {
	transport *mock.Transport
	lastSpawn float64
}

func (m *MasterTransport) Send(msg launce.Message) error {
	b, err := launce.EncodeMessage(msg)
	if err != nil {
		return err
	}
	return m.transport.Send(b)
}

func (m *MasterTransport) Receive() (launce.ReceivedMessage, error) {
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

func (m *MasterTransport) SendSpawn(users map[string]int64, nodeID string) error {
	m.lastSpawn += 1
	return m.Send(launce.Message{
		Type: worker.MessageSpawn,
		Data: worker.SpawnPayload{
			Timestamp:        m.lastSpawn,
			UserClassesCount: users,
			Host:             "",
			StopTimeout:      0,
			ParsedOptions:    parsedOptions,
		},
		NodeID: nodeID,
	})
}

func extractMessageData[T any](msg launce.ReceivedMessage) T {
	var v T
	if err := msgpack.Unmarshal(msg.Data, &v); err != nil {
		var z T
		return z
	}
	return v
}

func setupWorker(t *testing.T) (*launce.Worker, *MasterTransport) {
	t.Helper()

	masterTransport, workerTransport, err := mock.MakeTransportSet()
	if err != nil {
		t.Fatal(err)
	}

	w, err := launce.NewWorker(workerTransport)
	if err != nil {
		t.Fatal(err)
	}

	w.HeartbeatInterval = 0
	w.StatsReportInterval = 0
	w.MetricsMonitorInterval = 0

	return w, &MasterTransport{transport: masterTransport}
}

func startMasterReceiver(wg *sync.WaitGroup, master *MasterTransport, filterMessages ...string) (<-chan launce.ReceivedMessage, func()) {
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
					Type:   worker.MessageAck,
					Data:   worker.AckPayload{Index: 1},
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

func Wait(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

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
	msg := extractMessageData[string](customMessage)

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
		ch <- "1:" + extractMessageData[string](msg)
	})
	w.RegisterMessage("custom-message", func(msg launce.ReceivedMessage) {
		ch <- "2:" + extractMessageData[string](msg)
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
		ch <- "1:" + extractMessageData[string](msg)
	})
	w.RegisterMessage("custom-message2", func(msg launce.ReceivedMessage) {
		ch <- "2:" + extractMessageData[string](msg)
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

	userFunc, stats, uc := mock.UserGenerator(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	w.RegisterUser("test-user", userFunc)

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

	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 3)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 5,
	}, w.ClientID)

	uc.WaitStart(5)

	if n := stats.GetStarted(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 1,
	}, w.ClientID)

	uc.WaitStop(4)

	if n := stats.GetStarted(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}
	if n := stats.GetStopped(); n != 4 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 4)
	}

	w.Quit()
	uc.WaitStop(5)

	if n := stats.GetStarted(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}
	if n := stats.GetStopped(); n != 5 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 5)
	}

	wg.Wait()
}

func TestWorker_SpawnMessage(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	masterCh, _ := startMasterReceiver(&wg, master, worker.MessageClientReady, worker.MessageSpawning, worker.MessageSpawningComplete, worker.MessageClientStopped)

	userFunc, _, uc := mock.UserGenerator(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	w.RegisterUser("test-user", userFunc)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	msg := <-masterCh
	if msg.Type != worker.MessageClientReady {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, worker.MessageClientReady)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 3,
	}, w.ClientID)

	msg = <-masterCh
	if msg.Type != worker.MessageSpawning {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, worker.MessageSpawning)
	}

	msg = <-masterCh
	if msg.Type != worker.MessageSpawningComplete {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, worker.MessageSpawningComplete)
	}

	uc.WaitStart(3)

	_ = master.Send(launce.Message{
		Type:   worker.MessageStop,
		Data:   nil,
		NodeID: w.ClientID,
	})

	msg = <-masterCh
	if msg.Type != worker.MessageClientStopped {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, worker.MessageClientStopped)
	}

	uc.WaitStop(3)

	msg = <-masterCh
	if msg.Type != worker.MessageClientReady {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, worker.MessageClientReady)
	}

	w.Quit()

	wg.Wait()
}

func TestWorker_QuitMessage(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	masterCh, waitForReady := startMasterReceiver(&wg, master, worker.MessageClientReady)

	userFunc, _, _ := mock.UserGenerator(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	w.RegisterUser("test-user", userFunc)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	waitForReady()

	msg := <-masterCh
	if msg.Type != worker.MessageClientReady {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, worker.MessageClientReady)
	}

	_ = master.Send(launce.Message{
		Type:   worker.MessageQuit,
		Data:   nil,
		NodeID: w.ClientID,
	})

	<-masterCh

	wg.Wait()
}

func TestWorker_ExceptionMessage(t *testing.T) {
	var wg sync.WaitGroup

	w, master := setupWorker(t)
	masterCh, waitForReady := startMasterReceiver(&wg, master, worker.MessageException)

	errTest := errors.New("test error")

	var first atomic.Bool
	first.Store(true)
	userFunc, _, uc := mock.UserGenerator(func(ctx context.Context, u *mock.User) error {
		if first.Swap(false) {
			u.Runner().ReportException(errTest)
			return nil
		}
		return Wait(ctx)
	})

	w.RegisterUser("test-user", userFunc)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Join()
	}()

	waitForReady()

	_ = master.SendSpawn(map[string]int64{
		"test-user": 3,
	}, w.ClientID)

	uc.WaitStart(1)

	msg := <-masterCh

	w.Quit()

	data := extractMessageData[worker.ExceptionPayload](msg)
	if data.Msg != errTest.Error() {
		t.Fatalf("unexpected exception message. got:%v want:%v", data.Msg, errTest.Error())
	}

	wg.Wait()
}
