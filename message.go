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

	"github.com/vmihailenco/msgpack/v5"
)

type Message struct {
	Type   string
	Data   any
	NodeID string
}

type ReceivedMessage struct {
	Type   string
	Data   msgpack.RawMessage
	NodeID string
}

func (r *ReceivedMessage) DecodePayload(v interface{}) error {
	return msgpack.Unmarshal(r.Data, v)
}

func encodeMessage(msg Message) ([]byte, error) {
	b := bytes.NewBuffer(nil)
	enc := msgpack.NewEncoder(b)
	if err := enc.EncodeArrayLen(3); err != nil {
		return nil, err
	}
	if err := enc.EncodeString(msg.Type); err != nil {
		return nil, err
	}
	if err := enc.Encode(msg.Data); err != nil {
		return nil, err
	}
	if err := enc.EncodeString(msg.NodeID); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func decodeMessage(data []byte) (ReceivedMessage, error) {
	var msg ReceivedMessage
	if err := msgpack.Unmarshal(data, &msg); err != nil {
		return ReceivedMessage{}, err
	}
	return msg, nil
}
