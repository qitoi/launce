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

	worker, err := launce.NewWorker(workerTransport)
	if err != nil {
		t.Fatal(err)
	}

	worker.HeartbeatInterval = 0
	worker.StatsReportInterval = 0
	worker.MetricsMonitorInterval = 0

	return worker, &MasterTransport{transport: masterTransport}
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

func TestWorker_Join(t *testing.T) {
	var wg sync.WaitGroup

	worker, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	wg.Add(1)
	go func() {
		defer wg.Done()
		waitForReady()
		worker.Quit()
	}()

	if err := worker.Join(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	wg.Wait()
}

func TestWorker_Quit(t *testing.T) {
	var wg sync.WaitGroup

	worker, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Join()
	}()

	waitForReady()

	worker.Quit()

	wg.Wait()
}

func TestWorker_RegisterMessage(t *testing.T) {
	var wg sync.WaitGroup

	worker, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	ch := make(chan launce.ReceivedMessage)
	worker.RegisterMessage("custom-message", func(msg launce.ReceivedMessage) {
		ch <- msg
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Join()
	}()

	waitForReady()

	_ = master.Send(launce.Message{
		Type:   "custom-message",
		Data:   "hello",
		NodeID: worker.ClientID,
	})

	customMessage := <-ch
	msg := extractMessageData[string](customMessage)

	if customMessage.Type != "custom-message" {
		t.Fatalf("received custome message type mismatch. got:%v want:%v", customMessage.Type, "custom-message")
	}
	if customMessage.NodeID != worker.ClientID {
		t.Fatalf("received custome message node_id mismatch. got:%v want:%v", customMessage.NodeID, worker.ClientID)
	}
	if msg != "hello" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg, "hello")
	}

	worker.Quit()

	wg.Wait()
}

func TestWorker_RegisterMessage_MultipleReceivers(t *testing.T) {
	var wg sync.WaitGroup

	worker, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	ch := make(chan string)
	worker.RegisterMessage("custom-message", func(msg launce.ReceivedMessage) {
		ch <- "1:" + extractMessageData[string](msg)
	})
	worker.RegisterMessage("custom-message", func(msg launce.ReceivedMessage) {
		ch <- "2:" + extractMessageData[string](msg)
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Join()
	}()

	waitForReady()

	_ = master.Send(launce.Message{
		Type:   "custom-message",
		Data:   "hello",
		NodeID: worker.ClientID,
	})

	msg1 := <-ch
	if msg1 != "1:hello" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg1, "1:hello")
	}
	msg2 := <-ch
	if msg2 != "2:hello" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg2, "2:hello")
	}

	worker.Quit()

	wg.Wait()
}

func TestWorker_RegisterMessage_MultipleMessages(t *testing.T) {
	var wg sync.WaitGroup

	worker, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	ch := make(chan string, 1)
	worker.RegisterMessage("custom-message1", func(msg launce.ReceivedMessage) {
		ch <- "1:" + extractMessageData[string](msg)
	})
	worker.RegisterMessage("custom-message2", func(msg launce.ReceivedMessage) {
		ch <- "2:" + extractMessageData[string](msg)
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Join()
	}()

	waitForReady()

	_ = master.Send(launce.Message{
		Type:   "custom-message1",
		Data:   "foo",
		NodeID: worker.ClientID,
	})
	_ = master.Send(launce.Message{
		Type:   "custom-message2",
		Data:   "bar",
		NodeID: worker.ClientID,
	})

	msg1 := <-ch
	if msg1 != "1:foo" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg1, "1:foo")
	}
	msg2 := <-ch
	if msg2 != "2:bar" {
		t.Fatalf("received custome message mismatch. got:%v want:%v", msg2, "2:bar")
	}

	worker.Quit()

	wg.Wait()
}

func TestWorker_RegisterUser(t *testing.T) {
	var wg sync.WaitGroup

	worker, master := setupWorker(t)
	_, waitForReady := startMasterReceiver(&wg, master)

	userFunc, stats, uc := mock.UserGenerator(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	worker.RegisterUser("test-user", userFunc)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Join()
	}()

	waitForReady()

	_ = master.SendSpawn(map[string]int64{
		"test-user": 3,
	}, worker.ClientID)

	uc.WaitStart(3)

	if n := stats.GetStarted(); n != 3 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 3)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 5,
	}, worker.ClientID)

	uc.WaitStart(5)

	if n := stats.GetStarted(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 1,
	}, worker.ClientID)

	uc.WaitStop(4)

	if n := stats.GetStarted(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}
	if n := stats.GetStopped(); n != 4 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 4)
	}

	worker.Quit()
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

	worker, master := setupWorker(t)
	masterCh, _ := startMasterReceiver(&wg, master, launce.MessageClientReady, launce.MessageSpawning, launce.MessageSpawningComplete, launce.MessageClientStopped)

	userFunc, _, uc := mock.UserGenerator(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	worker.RegisterUser("test-user", userFunc)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Join()
	}()

	msg := <-masterCh
	if msg.Type != launce.MessageClientReady {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageClientReady)
	}

	_ = master.SendSpawn(map[string]int64{
		"test-user": 3,
	}, worker.ClientID)

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
		NodeID: worker.ClientID,
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

	worker.Quit()

	wg.Wait()
}

func TestWorker_QuitMessage(t *testing.T) {
	var wg sync.WaitGroup

	worker, master := setupWorker(t)
	masterCh, waitForReady := startMasterReceiver(&wg, master, launce.MessageClientReady)

	userFunc, _, _ := mock.UserGenerator(func(ctx context.Context, u *mock.User) error {
		return Wait(ctx)
	})

	worker.RegisterUser("test-user", userFunc)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Join()
	}()

	waitForReady()

	msg := <-masterCh
	if msg.Type != launce.MessageClientReady {
		t.Fatalf("unexpected master received message. got:%v want:%v", msg.Type, launce.MessageClientReady)
	}

	_ = master.Send(launce.Message{
		Type:   launce.MessageQuit,
		Data:   nil,
		NodeID: worker.ClientID,
	})

	<-masterCh

	wg.Wait()
}

func TestWorker_ExceptionMessage(t *testing.T) {
	var wg sync.WaitGroup

	worker, master := setupWorker(t)
	masterCh, waitForReady := startMasterReceiver(&wg, master, launce.MessageException)

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

	worker.RegisterUser("test-user", userFunc)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Join()
	}()

	waitForReady()

	_ = master.SendSpawn(map[string]int64{
		"test-user": 3,
	}, worker.ClientID)

	uc.WaitStart(1)

	msg := <-masterCh

	worker.Quit()

	data := extractMessageData[launce.ExceptionPayload](msg)
	if data.Msg != errTest.Error() {
		t.Fatalf("unexpected exception message. got:%v want:%v", data.Msg, errTest.Error())
	}

	wg.Wait()
}
