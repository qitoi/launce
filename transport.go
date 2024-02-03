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
	"bytes"
	"context"
	"fmt"

	"github.com/go-zeromq/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

type Transport interface {
	Open(ctx context.Context, clientID string) error
	Close() error
	Send(msg Message) error
	Receive() (ParsedMessage, error)
}

var (
	_ Transport = (*ZmqTransport)(nil)
)

type ZmqTransport struct {
	Host string
	Port int

	opts   []zmq4.Option
	socket zmq4.Socket
}

func NewZmqTransport(host string, port int, opts ...zmq4.Option) *ZmqTransport {
	return &ZmqTransport{
		Host: host,
		Port: port,

		opts: opts,
	}
}

func (t *ZmqTransport) Open(ctx context.Context, clientID string) error {
	opts := append(t.opts, zmq4.WithID(zmq4.SocketIdentity(clientID)))
	socket := zmq4.NewDealer(ctx, opts...)
	err := socket.Dial(fmt.Sprintf("tcp://%s:%d", t.Host, t.Port))
	if err != nil {
		return err
	}
	t.socket = socket
	return nil
}

func (t *ZmqTransport) Close() error {
	return t.socket.Close()
}

func (t *ZmqTransport) Send(msg Message) error {
	b, err := msg.Encode()
	if err != nil {
		return err
	}
	return t.socket.Send(zmq4.NewMsg(b))
}

func (t *ZmqTransport) Receive() (ParsedMessage, error) {
	msg, err := t.socket.Recv()
	if err != nil {
		return ParsedMessage{}, err
	}

	parsed, err := ParseMessage(msg.Bytes())
	if err != nil {
		return ParsedMessage{}, err
	}

	return parsed, nil
}

type Message struct {
	Type   string `msgpack:"type"`
	Data   any    `msgpack:"data"`
	NodeID string `msgpack:"node_id"`
}

func (m *Message) Encode() ([]byte, error) {
	b := bytes.NewBuffer(nil)
	enc := msgpack.NewEncoder(b)
	if err := enc.EncodeArrayLen(3); err != nil {
		return nil, err
	}
	if err := enc.EncodeString(m.Type); err != nil {
		return nil, err
	}
	if err := enc.Encode(m.Data); err != nil {
		return nil, err
	}
	if err := enc.EncodeString(m.NodeID); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type ParsedMessage struct {
	Type   string             `msgpack:"type"`
	Data   msgpack.RawMessage `msgpack:"data"`
	NodeID string             `msgpack:"node_id"`
}

func ParseMessage(data []byte) (ParsedMessage, error) {
	var msg ParsedMessage
	dec := msgpack.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&msg); err != nil {
		return ParsedMessage{}, err
	}
	return msg, nil
}
