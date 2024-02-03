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
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type WorkerState = int64

const (
	WorkerStateInit WorkerState = iota
	WorkerStateSpawning
	WorkerStateRunning
	WorkerStateCleanup
	WorkerStateStopped
)

var (
	ErrConnection = errors.New("failed to connect to master")
)

const (
	defaultHeartbeatInterval      = 1 * time.Second
	defaultMetricsMonitorInterval = 5 * time.Second
	defaultStatsReportInterval    = 3 * time.Second
	defaultMasterHeartbeatTimeout = 60 * time.Second

	connectTimeout    = 5 * time.Second
	connectRetryCount = 60
)

var (
	workerStateNames = map[WorkerState]string{
		WorkerStateInit:     "ready",
		WorkerStateSpawning: "spawning",
		WorkerStateRunning:  "running",
		WorkerStateCleanup:  "cleanup",
		WorkerStateStopped:  "stopped",
	}
)

func generateClientID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return hostname + "_" + uuid.NewString(), nil
}

type MessageHandler func(msg ReceivedMessage)

type Worker struct {
	Version                string
	ClientID               string
	HeartbeatInterval      time.Duration
	MetricsMonitorInterval time.Duration
	StatsReportInterval    time.Duration
	MasterHeartbeatTimeout time.Duration

	runner          *Runner
	cancel          atomic.Value
	index           int64
	state           WorkerState
	transport       Transport
	spawnCh         chan map[string]int64
	ackCh           chan struct{}
	heartbeatCh     chan struct{}
	messageHandlers map[string][]MessageHandler
	procInfo        *ProcessInfo
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
		ackCh:           make(chan struct{}, 1),
		heartbeatCh:     make(chan struct{}),
		messageHandlers: make(map[string][]MessageHandler),
		procInfo:        procInfo,
	}

	worker.runner.ExceptionReporter = worker.reportException

	return worker, nil
}

func (w *Worker) Join() error {
	var wg sync.WaitGroup

	// マスターへのコネクション確立前に Quit された場合は、トランスポートのコンテキストをキャンセルし、トランスポートを閉じることで終了させる
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel.Store(cancel)
	if err := w.open(ctx); err != nil {
		return err
	}

	// マスターへのコネクション確立後に Quit された場合は、トランスポートを閉じる前に quit メッセージを送るため
	// トランスポートは閉じずに goroutine 終了のためのコンテキストキャンセルに切り替える
	ctx, cancel = context.WithCancel(context.Background())
	w.cancel.Store(cancel)

	w.startMessageProcess(&wg)

	// マスターへの client_ready 送信、 ack 受信による疎通の確認
	if err := w.connect(ctx); err != nil {
		_ = w.close()
		return err
	}

	var procWg sync.WaitGroup

	w.startHeartbeatProcess(ctx, &procWg, w.HeartbeatInterval)
	w.startHeartbeatCheckProcess(ctx, &procWg, w.MasterHeartbeatTimeout)
	w.startMetricsMonitorProcess(ctx, &procWg, w.MetricsMonitorInterval)
	w.startStatsProcess(ctx, &procWg, w.StatsReportInterval)
	w.startSpawnProcess(ctx, &procWg)

	procWg.Wait()

	// ワーカーの終了時はマスターに quit メッセージを送信する
	if err := w.send(MessageQuit, nil); err != nil {
		_ = w.close()
		return err
	}

	if err := w.close(); err != nil {
		return err
	}

	wg.Wait()

	return nil
}

func (w *Worker) Stop() {
	if atomic.LoadInt64(&w.state) == WorkerStateStopped {
		return
	}
	atomic.StoreInt64(&w.state, WorkerStateCleanup)
	w.runner.Stop()
	atomic.StoreInt64(&w.state, WorkerStateStopped)
}

func (w *Worker) Quit() {
	w.Stop()
	if f := w.cancel.Load(); f != nil {
		if cancel, ok := f.(context.CancelFunc); ok {
			cancel()
		}
	}
}

func (w *Worker) open(ctx context.Context) error {
	return w.transport.Open(ctx, w.ClientID)
}

func (w *Worker) close() error {
	return w.transport.Close()
}

func (w *Worker) connect(ctx context.Context) error {
	retry := 0
	ticker := time.NewTicker(connectTimeout)
	defer ticker.Stop()

clear:
	for {
		select {
		case <-w.ackCh:
		default:
			break clear
		}
	}

	if err := w.send(MessageClientReady, w.Version); err != nil {
		return err
	}

loop:
	for {
		select {
		case <-ticker.C:
			retry += 1
			if retry > connectRetryCount {
				return ErrConnection
			}
			break

		case <-w.ackCh:
			break loop

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (w *Worker) startMessageProcess(wg *sync.WaitGroup) {
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			msg, err := w.recv()
			if err != nil {
				w.Quit()
				return
			}

			switch msg.Type {
			case MessageAck:
				var payload AckPayload
				if err := msg.Decode(&payload); err != nil {
					return
				}
				atomic.StoreInt64(&w.index, payload.Index)

				select {
				case w.ackCh <- struct{}{}:
				default:
				}

				break

			case MessageSpawn:
				var payload SpawnPayload
				if err := msg.Decode(&payload); err != nil {
					return
				}

				parsedOptions, err := NewParsedOptions(payload.ParsedOptions)
				if err != nil {
					return
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
				_ = w.close()
				_ = w.open(context.Background())
				break

			case MessageHeartbeat:
				select {
				case w.heartbeatCh <- struct{}{}:
				default:
				}

			case MessageQuit:
				w.Stop()
				_ = w.sendStats()
				w.Quit()
				break

			default:
				if handlers, ok := w.messageHandlers[msg.Type]; ok {
					for _, handler := range handlers {
						handler(msg)
					}
				}
				break
			}
		}
	}()
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

func (w *Worker) startSpawnProcess(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()

	loop:
		for {
			select {
			case spawnCount := <-w.spawnCh:
				atomic.StoreInt64(&w.state, WorkerStateSpawning)
				_ = w.send(MessageSpawning, nil)

				users := w.runner.Users()
				var total int64
				for _, u := range users {
					total += u
				}

				payload := &SpawningCompletePayload{
					UserClassesCount: users,
					UserCount:        total,
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

			case <-ctx.Done():
				break loop
			}
		}
	}()
}

func (w *Worker) startHeartbeatProcess(ctx context.Context, wg *sync.WaitGroup, interval time.Duration) {
	if interval <= 0 {
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

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

			case <-ctx.Done():
				break loop
			}
		}
	}()
}

func (w *Worker) startHeartbeatCheckProcess(ctx context.Context, wg *sync.WaitGroup, timeout time.Duration) {
	wg.Add(1)
	go func() {
		defer wg.Done()

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
				w.Quit()
				break loop

			case <-ctx.Done():
				break loop
			}
		}
	}()
}

func (w *Worker) startStatsProcess(ctx context.Context, wg *sync.WaitGroup, interval time.Duration) {
	if interval <= 0 {
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

	loop:
		for {
			select {
			case <-ticker.C:
				_ = w.sendStats()
				break

			case <-ctx.Done():
				break loop
			}
		}
	}()
}

func (w *Worker) sendStats() error {
	payload := convertStatisticsPayload(w.runner.FlushStats())
	payload.UserClassesCount = w.runner.Users()
	for _, count := range payload.UserClassesCount {
		payload.UserCount += count
	}
	return w.send(MessageStats, payload)
}

func (w *Worker) startMetricsMonitorProcess(ctx context.Context, wg *sync.WaitGroup, interval time.Duration) {
	if interval <= 0 {
		return
	}

	_ = w.procInfo.Update()

	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

	loop:
		for {
			select {
			case <-ticker.C:
				_ = w.procInfo.Update()
				break

			case <-ctx.Done():
				break loop
			}
		}
	}()
}

func (w *Worker) recv() (ReceivedMessage, error) {
	b, err := w.transport.Receive()
	if err != nil {
		return ReceivedMessage{}, err
	}
	return parseMessage(b)
}

func (w *Worker) send(typ string, data any) error {
	msg := Message{
		Type:   typ,
		Data:   data,
		NodeID: w.ClientID,
	}
	b, err := encodeMessage(msg)
	if err != nil {
		return err
	}
	return w.transport.Send(b)
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
