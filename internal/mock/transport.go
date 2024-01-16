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

package mock

import (
	"context"
	"sync"

	"github.com/qitoi/launce"
)

var (
	_ launce.Transport = (*Transport)(nil)
)

type Transport struct {
	Dest   *Transport
	sendCh chan<- []byte
	recvCh <-chan []byte
	ctx    context.Context
	cancel func()
	mu     sync.RWMutex
	wg     sync.WaitGroup
}

func (m *Transport) Open(clientID string) error {
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

func (m *Transport) Close() error {
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

func (m *Transport) Send(msg launce.Message) error {
	b, err := msg.Encode()
	if err != nil {
		return err
	}

	m.mu.RLock()
	ch := m.sendCh
	m.mu.RUnlock()

	m.wg.Add(1)
	defer m.wg.Done()

	select {
	case ch <- b:
		break
	case <-m.ctx.Done():
		return launce.ErrConnectionClosed
	}

	return nil
}

func (m *Transport) Receive() (launce.ParsedMessage, error) {
	m.mu.RLock()
	ch := m.recvCh
	m.mu.RUnlock()

	select {
	case b, ok := <-ch:
		if !ok {
			_ = m.Close()
			return launce.ParsedMessage{}, launce.ErrConnectionClosed
		}
		return launce.ParseMessage(b)
	case <-m.ctx.Done():
		return launce.ParsedMessage{}, launce.ErrConnectionClosed
	}
}

func MakeTransportSet() (*Transport, *Transport, error) {
	t1 := &Transport{}
	t2 := &Transport{}
	t1.Dest, t2.Dest = t2, t1
	if err := t2.Open(""); err != nil {
		return nil, nil, err
	}
	return t1, t2, nil
}
