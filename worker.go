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
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/vmihailenco/msgpack/v5"
)

type WorkerState = int64

const (
	WorkerStateInit WorkerState = iota
	WorkerStateSpawning
	WorkerStateRunning
)

const (
	defaultHeartbeatInterval      = 1 * time.Second
	defaultMetricsMonitorInterval = 5 * time.Second
	defaultStatsReportInterval    = 3 * time.Second
	defaultMasterHeartbeatTimeout = 60 * time.Second
)

var (
	workerStateNames = map[WorkerState]string{
		WorkerStateInit:     "ready",
		WorkerStateSpawning: "spawning",
		WorkerStateRunning:  "running",
	}
)

func generateClientID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return hostname + "_" + uuid.NewString(), nil
}

type MessageHandler func(msg ParsedMessage)

type Worker struct {
	Version                string
	ClientID               string
	HeartbeatInterval      time.Duration
	MetricsMonitorInterval time.Duration
	StatsReportInterval    time.Duration
	MasterHeartbeatTimeout time.Duration

	runner          *Runner
	index           int64
	state           WorkerState
	transport       Transport
	spawnCh         chan map[string]int64
	heartbeatCh     chan struct{}
	messageHandlers map[string][]MessageHandler
	procInfo        *ProcessInfo
	closeCond       *sync.Cond
}

func NewWorker(transport Transport) (*Worker, error) {
	id, err := generateClientID()
	if err != nil {
		return nil, err
	}
	runner, err := NewRunner()
	if err != nil {
		return nil, err
	}
	procInfo, err := NewProcessInfo(os.Getpid())
	if err != nil {
		return nil, err
	}
	worker := &Worker{
		Version:                "launce-0.0.1",
		ClientID:               id,
		HeartbeatInterval:      defaultHeartbeatInterval,
		MetricsMonitorInterval: defaultMetricsMonitorInterval,
		StatsReportInterval:    defaultStatsReportInterval,
		MasterHeartbeatTimeout: defaultMasterHeartbeatTimeout,

		runner:          runner,
		index:           -1,
		state:           WorkerStateInit,
		transport:       transport,
		spawnCh:         make(chan map[string]int64),
		heartbeatCh:     make(chan struct{}),
		messageHandlers: make(map[string][]MessageHandler),
		procInfo:        procInfo,
		closeCond:       sync.NewCond(&sync.Mutex{}),
	}

	worker.runner.ExceptionReporter = worker.reportException

	return worker, nil
}

func (w *Worker) Connect() error {
	if err := w.transport.Open(w.ClientID); err != nil {
		return err
	}

	closeCh := make(chan struct{})

	go func() {
		w.closeCond.L.Lock()
		w.closeCond.Wait()
		w.closeCond.L.Unlock()
		close(closeCh)
	}()

	if err := w.send(MessageClientReady, w.Version); err != nil {
		return err
	}

	go w.heartbeatCheckProcess(w.MasterHeartbeatTimeout, closeCh)
	go w.heartbeatProcess(w.HeartbeatInterval, closeCh)
	go w.metricsMonitorProcess(w.MetricsMonitorInterval, closeCh)
	go w.statsProcess(w.StatsReportInterval, closeCh)
	go w.spawn(closeCh)

	return nil
}

func (w *Worker) Close() error {
	w.closeCond.L.Lock()
	w.closeCond.Broadcast()
	w.closeCond.L.Unlock()

	w.runner.Stop()

	return w.transport.Close()
}

func (w *Worker) Quit() error {
	if err := w.send(MessageQuit, nil); err != nil {
		return err
	}
	return w.Close()
}

func (w *Worker) Start() error {
	for {
		if err := w.process(); err != nil {
			return err
		}
	}
}

func (w *Worker) RegisterUser(name string, f UserGenerator) {
	w.runner.RegisterUser(name, f)
}

func (w *Worker) RegisterMessage(typ string, handler MessageHandler) {
	w.messageHandlers[typ] = append(w.messageHandlers[typ], handler)
}

func (w *Worker) Index() int64 {
	return atomic.LoadInt64(&w.index)
}

func (w *Worker) State() WorkerState {
	return atomic.LoadInt64(&w.state)
}

func (w *Worker) reportException(err error) {
	trace := ""
	var e Error
	if errors.As(err, &e) {
		trace = e.Traceback()
	} else {
		trace = ""
	}
	_ = w.send(MessageException, ExceptionPayload{
		Msg:       err.Error(),
		Traceback: trace,
	})
}

func (w *Worker) process() error {
	msg, err := w.transport.Receive()
	if err != nil {
		return err
	}

	switch msg.Type {
	case MessageAck:
		var payload AckPayload
		if err := msgpack.Unmarshal(msg.Data, &payload); err != nil {
			return err
		}
		atomic.StoreInt64(&w.index, payload.Index)
		break

	case MessageSpawn:
		var payload SpawnPayload
		if err := msgpack.Unmarshal(msg.Data, &payload); err != nil {
			return err
		}

		parsedOptions, err := NewParsedOptions(payload.ParsedOptions)
		if err != nil {
			return err
		}

		w.runner.SetParsedOptions(parsedOptions)

		state := atomic.LoadInt64(&w.state)
		if state != WorkerStateRunning && state != WorkerStateSpawning {
			w.runner.FlushStats()
			w.runner.Start()
		}

		w.spawnCh <- payload.UserClassesCount
		break

	case MessageStop:
		w.runner.SetParsedOptions(nil)
		w.runner.Stop()
		_ = w.send(MessageClientStopped, nil)
		atomic.StoreInt64(&w.state, WorkerStateInit)
		_ = w.send(MessageClientReady, w.Version)
		break

	case MessageReconnect:
		_ = w.Close()
		_ = w.Connect()
		break

	case MessageHeartbeat:
		select {
		case w.heartbeatCh <- struct{}{}:
		default:
		}

	case MessageQuit:
		_ = w.Close()
		_ = w.sendStats()
		break

	default:
		if handlers, ok := w.messageHandlers[msg.Type]; ok {
			for _, handler := range handlers {
				handler(msg)
			}
		}
		break
	}

	return nil
}

func (w *Worker) spawn(closeCh chan struct{}) {
loop:
	for {
		select {
		case spawnCount := <-w.spawnCh:
			atomic.StoreInt64(&w.state, WorkerStateSpawning)
			_ = w.send(MessageSpawning, nil)

			payload := &SpawningCompletePayload{
				UserClassesCount: map[string]int64{},
				UserCount:        0,
			}

			for name, count := range spawnCount {
				if err := w.runner.Spawn(name, int(count)); err != nil {
					payload.UserCount += count
					payload.UserClassesCount[name] = count
				}
			}

			_ = w.send(MessageSpawningComplete, payload)

			atomic.StoreInt64(&w.state, WorkerStateRunning)

			break

		case <-closeCh:
			break loop
		}
	}
}

func (w *Worker) heartbeatProcess(interval time.Duration, closeCh chan struct{}) {
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			state := atomic.LoadInt64(&w.state)
			msg := HeartbeatPayload{
				State:              workerStateNames[state],
				CurrentCPUUsage:    w.procInfo.CPUUsage(),
				CurrentMemoryUsage: w.procInfo.MemoryUsage(),
			}
			_ = w.send(MessageHeartbeat, msg)
			break

		case <-closeCh:
			break loop
		}
	}
}

func (w *Worker) heartbeatCheckProcess(timeout time.Duration, closeCh chan struct{}) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

loop:
	for {
		select {
		case <-w.heartbeatCh:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(timeout)
			break

		case <-timer.C:
			_ = w.Quit()
			break loop

		case <-closeCh:
			break loop
		}
	}
}

func (w *Worker) statsProcess(interval time.Duration, closeCh chan struct{}) {
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			_ = w.sendStats()
			break

		case <-closeCh:
			break loop
		}
	}
}

func (w *Worker) sendStats() error {
	payload := convertStatisticsPayload(w.runner.FlushStats())
	payload.UserClassesCount = w.runner.Users()
	for _, count := range payload.UserClassesCount {
		payload.UserCount += count
	}
	return w.send(MessageStats, payload)
}

func (w *Worker) metricsMonitorProcess(interval time.Duration, closeCh chan struct{}) {
	if interval <= 0 {
		return
	}

	_ = w.procInfo.Update()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			_ = w.procInfo.Update()
			break

		case <-closeCh:
			break loop
		}
	}
}

func (w *Worker) send(typ string, data any) error {
	msg := Message{
		Type:   typ,
		Data:   data,
		NodeID: w.ClientID,
	}
	return w.transport.Send(msg)
}

func convertStatisticsPayload(entries StatisticsEntries, total *StatisticsEntry, errors StatisticsErrors) *StatsPayload {
	payload := &StatsPayload{
		Stats:  make([]*StatsPayloadEntry, len(entries)),
		Errors: make(map[string]*StatsPayloadError, len(errors)),
	}

	var idx int
	for key, entry := range entries {
		payload.Stats[idx] = convertStatisticsEntry(key.Name, key.Method, entry)
		idx += 1
	}

	payload.StatsTotal = convertStatisticsEntry("Aggregated", "", total)

	for key, occurrence := range errors {
		payload.Errors[key.Encode()] = convertStatisticsError(key, occurrence)
	}

	return payload
}

func convertStatisticsEntry(name, method string, entry *StatisticsEntry) *StatsPayloadEntry {
	return &StatsPayloadEntry{
		Name:                 name,
		Method:               method,
		LastRequestTimestamp: float64(entry.lastRequestTimestamp.UnixNano()) / 1e9, // [s]
		StartTime:            float64(entry.startTime.UnixNano()) / 1e9,            // [s]
		NumRequests:          entry.numRequests,
		NumNoneRequests:      entry.numNoneRequests,
		NumFailures:          entry.numFailures,
		TotalResponseTime:    float64(entry.totalResponseTime.Nanoseconds()) / 1e6, // [ms]
		MaxResponseTime:      float64(entry.maxResponseTime.Nanoseconds()) / 1e6,   // [ms]
		MinResponseTime:      float64(entry.minResponseTime.Nanoseconds()) / 1e6,   // [ms]
		TotalContentLength:   entry.totalContentLength,
		ResponseTimes:        entry.responseTimes,
		NumReqsPerSec:        entry.numRequestsPerSec,
		NumFailPerSec:        entry.numFailuresPerSec,
	}
}

func convertStatisticsError(key StatisticsErrorKey, occurrence int64) *StatsPayloadError {
	return &StatsPayloadError{
		Name:        key.Name,
		Method:      key.Method,
		Error:       key.Error,
		Occurrences: occurrence,
	}
}
