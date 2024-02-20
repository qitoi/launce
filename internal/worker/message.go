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

package worker

import (
	"github.com/vmihailenco/msgpack/v5"
)

const (
	// message type

	MessageHeartbeat = "heartbeat"
	MessageQuit      = "quit"

	// master to worker message type

	MessageAck       = "ack"
	MessageSpawn     = "spawn"
	MessageStop      = "stop"
	MessageReconnect = "reconnect"

	// worker to master message type

	MessageClientReady      = "client_ready"
	MessageClientStopped    = "client_stopped"
	MessageSpawning         = "spawning"
	MessageSpawningComplete = "spawning_complete"
	MessageStats            = "stats"
	MessageException        = "exception"
)

type AckPayload struct {
	Index int64 `msgpack:"index"`
}

type SpawnPayload struct {
	Timestamp        float64            `msgpack:"timestamp"`
	UserClassesCount map[string]int64   `msgpack:"user_classes_count"`
	Host             string             `msgpack:"host"`
	StopTimeout      float64            `msgpack:"stop_timeout"`
	ParsedOptions    msgpack.RawMessage `msgpack:"parsed_options"`
}

type HeartbeatPayload struct {
	State              string  `msgpack:"state"`
	CurrentCPUUsage    float64 `msgpack:"current_cpu_usage"`
	CurrentMemoryUsage uint64  `msgpack:"current_memory_usage"`
}

type StatsPayloadEntry struct {
	Name                 string          `msgpack:"name"`
	Method               string          `msgpack:"method"`
	LastRequestTimestamp float64         `msgpack:"last_request_timestamp"`
	StartTime            float64         `msgpack:"start_time"`
	NumRequests          int64           `msgpack:"num_requests"`
	NumNoneRequests      int64           `msgpack:"num_none_requests"`
	NumFailures          int64           `msgpack:"num_failures"`
	TotalResponseTime    float64         `msgpack:"total_response_time"`
	MaxResponseTime      float64         `msgpack:"max_response_time"`
	MinResponseTime      *float64        `msgpack:"min_response_time"`
	TotalContentLength   int64           `msgpack:"total_content_length"`
	ResponseTimes        map[int64]int64 `msgpack:"response_times"`
	NumReqsPerSec        map[int64]int64 `msgpack:"num_reqs_per_sec"`
	NumFailPerSec        map[int64]int64 `msgpack:"num_fail_per_sec"`
}

type StatsPayloadError struct {
	Name        string `msgpack:"name"`
	Method      string `msgpack:"method"`
	Error       string `msgpack:"error"`
	Occurrences int64  `msgpack:"occurrences"`
}

type StatsPayload struct {
	Stats            []*StatsPayloadEntry          `msgpack:"stats"`
	StatsTotal       *StatsPayloadEntry            `msgpack:"stats_total"`
	Errors           map[string]*StatsPayloadError `msgpack:"errors"`
	UserClassesCount map[string]int64              `msgpack:"user_classes_count"`
	UserCount        int64                         `msgpack:"user_count"`
}

type ExceptionPayload struct {
	Msg       string `msgpack:"msg"`
	Traceback string `msgpack:"traceback"`
}

type SpawningCompletePayload struct {
	UserClassesCount map[string]int64 `msgpack:"user_classes_count"`
	UserCount        int64            `msgpack:"user_count"`
}
