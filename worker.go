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

	"github.com/qitoi/launce/stats"
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
	defaultMasterHeartbeatTimeout = 60 * time.Second
	defaultStatsReportInterval    = 3 * time.Second
	defaultStatsAggregateInterval = 100 * time.Millisecond

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

type Worker struct {
	Version                string
	ClientID               string
	HeartbeatInterval      time.Duration
	MetricsMonitorInterval time.Duration
	MasterHeartbeatTimeout time.Duration
	StatsReportInterval    time.Duration
	StatsAggregateInterval time.Duration

	runner      *LoadRunner
	cancel      atomic.Value
	index       int64
	state       WorkerState
	transport   Transport
	stats       *stats.Stats
	spawnCh     chan map[string]int64
	ackCh       chan struct{}
	heartbeatCh chan struct{}
	procInfo    *processInfo
}

func NewWorker(transport Transport) (*Worker, error) {
	id, err := generateClientID()
	if err != nil {
		return nil, err
	}
	procInfo, err := newProcessInfo(os.Getpid())
	if err != nil {
		return nil, err
	}

	w := &Worker{
		Version:                "launce-0.0.1",
		ClientID:               id,
		HeartbeatInterval:      defaultHeartbeatInterval,
		MetricsMonitorInterval: defaultMetricsMonitorInterval,
		MasterHeartbeatTimeout: defaultMasterHeartbeatTimeout,
		StatsReportInterval:    defaultStatsReportInterval,
		StatsAggregateInterval: defaultStatsAggregateInterval,

		runner:      NewLoadRunner(),
		index:       -1,
		state:       WorkerStateInit,
		transport:   transport,
		stats:       stats.New(),
		spawnCh:     make(chan map[string]int64),
		ackCh:       make(chan struct{}, 1),
		heartbeatCh: make(chan struct{}),
		procInfo:    procInfo,
	}

	w.runner.SendMessageFunc = w.SendMessage
	w.runner.ReportExceptionFunc = w.reportException

	return w, nil
}

func (w *Worker) Join() error {
	var wg sync.WaitGroup

	w.runner.StatsNotifyInterval = w.StatsAggregateInterval

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
	if err := w.SendMessage(messageQuit, nil); err != nil {
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

	if err := w.SendMessage(messageClientReady, w.Version); err != nil {
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

		var lastReceivedSpawnTimestamp float64

		for {
			msg, err := w.recv()
			if err != nil {
				w.Quit()
				return
			}

			switch msg.Type {
			case messageAck:
				var payload ackPayload
				if err := msg.DecodePayload(&payload); err != nil {
					return
				}
				atomic.StoreInt64(&w.index, payload.Index)

				select {
				case w.ackCh <- struct{}{}:
				default:
				}

			case messageSpawn:
				var payload spawnPayload
				if err := msg.DecodePayload(&payload); err != nil {
					return
				}

				if payload.Timestamp <= lastReceivedSpawnTimestamp {
					break
				}
				lastReceivedSpawnTimestamp = payload.Timestamp

				w.runner.SetHost(payload.Host)
				w.runner.SetParsedOptions(&payload.ParsedOptions)

				w.spawnCh <- payload.UserClassesCount

			case messageStop:
				w.runner.SetParsedOptions(nil)
				w.runner.Stop()
				_ = w.SendMessage(messageClientStopped, nil)
				atomic.StoreInt64(&w.state, WorkerStateInit)
				_ = w.SendMessage(messageClientReady, w.Version)

			case messageReconnect:
				_ = w.close()
				_ = w.open(context.Background())

			case messageHeartbeat:
				select {
				case w.heartbeatCh <- struct{}{}:
				default:
				}

			case messageQuit:
				w.Stop()
				_ = w.sendStats()
				w.Quit()

			default:
				w.runner.HandleMessage(msg)
			}
		}
	}()
}

func (w *Worker) RegisterUser(name string, f func() User) {
	w.runner.RegisterUser(name, f)
}

func (w *Worker) RegisterMessage(typ string, handler MessageHandler) {
	w.runner.RegisterMessage(typ, handler)
}

func (w *Worker) OnTestStart(f func(ctx context.Context) error) {
	w.runner.OnTestStart(f)
}

func (w *Worker) OnTestStop(f func(ctx context.Context)) {
	w.runner.OnTestStop(f)
}

func (w *Worker) Index() int64 {
	return atomic.LoadInt64(&w.index)
}

func (w *Worker) State() WorkerState {
	return atomic.LoadInt64(&w.state)
}

func (w *Worker) SendMessage(typ string, data any) error {
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

func (w *Worker) reportException(err error) {
	trace := ""
	var e wrapError
	if errors.As(err, &e) {
		trace = e.StackTrace()
	} else {
		trace = ""
	}
	_ = w.SendMessage(messageException, exceptionPayload{
		Msg:       err.Error(),
		Traceback: trace,
	})
}

func (w *Worker) startSpawnProcess(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(2)

	relayCh := make(chan map[string]int64)

	// Spawn に時間が掛かる場合もメッセージ受信の goroutine を止めないために中継用の goroutine を間に挟む
	go func() {
		defer wg.Done()

		var ch chan map[string]int64
		var spawnCount map[string]int64

	loop:
		for {
			select {
			case spawnCount = <-w.spawnCh:
				ch = relayCh

			case ch <- spawnCount:
				spawnCount = nil
				ch = nil

			case <-ctx.Done():
				break loop
			}
		}

	}()

	go func() {
		defer wg.Done()

	loop:
		for {
			select {
			case spawnCount := <-relayCh:
				// Runner がスタートしていない状態で spawn がリクエストされた場合は Runner をスタートする
				if state := atomic.LoadInt64(&w.state); state != WorkerStateRunning && state != WorkerStateSpawning {
					// Runner の Start 中に停止された場合は spawn を中断する
					if err := w.runner.Start(); err != nil {
						continue
					}
				}

				atomic.StoreInt64(&w.state, WorkerStateSpawning)
				_ = w.SendMessage(messageSpawning, nil)

				users := w.runner.Users()
				var total int64
				for _, u := range users {
					total += u
				}

				payload := &spawningCompletePayload{
					UserClassesCount: make(map[string]int64),
					UserCount:        0,
				}
				for name, count := range spawnCount {
					if err := w.runner.Spawn(name, int(count)); err == nil {
						payload.UserCount += count
						payload.UserClassesCount[name] = count
					}
				}

				_ = w.SendMessage(messageSpawningComplete, payload)

				atomic.StoreInt64(&w.state, WorkerStateRunning)

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
				msg := heartbeatPayload{
					State:              workerStateNames[state],
					CurrentCPUUsage:    w.procInfo.CPUUsage(),
					CurrentMemoryUsage: w.procInfo.MemoryUsage(),
				}
				_ = w.SendMessage(messageHeartbeat, msg)

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

	ch := w.runner.Stats()

	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

	loop:
		for {
			select {
			case st := <-ch:
				w.stats.Merge(st)

			case <-ticker.C:
				_ = w.sendStats()

			case <-ctx.Done():
				break loop
			}
		}
	}()
}

func (w *Worker) sendStats() error {
	payload := convertStatisticsPayload(w.stats.Flush())
	payload.UserClassesCount = w.runner.Users()
	for _, count := range payload.UserClassesCount {
		payload.UserCount += count
	}
	return w.SendMessage(messageStats, payload)
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
	return decodeMessage(b)
}

func convertStatisticsPayload(entries stats.Entries, total *stats.Entry, errors stats.Errors) *statsPayload {
	payload := &statsPayload{
		Stats:  make([]*statsPayloadEntry, len(entries)),
		Errors: make(map[string]*statsPayloadError, len(errors)),
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

func convertStatisticsEntry(name, method string, entry *stats.Entry) *statsPayloadEntry {
	var minResponseTime *float64
	if entry.MinResponseTime != nil {
		minResponseTime = new(float64)
		*minResponseTime = float64(entry.MinResponseTime.Nanoseconds()) / 1e6 // [ms]
	}
	return &statsPayloadEntry{
		Name:                 name,
		Method:               method,
		LastRequestTimestamp: float64(entry.LastRequestTimestamp) / 1e9, // [s]
		StartTime:            float64(entry.StartTime) / 1e9,            // [s]
		NumRequests:          entry.NumRequests,
		NumNoneRequests:      entry.NumNoneRequests,
		NumFailures:          entry.NumFailures,
		TotalResponseTime:    float64(entry.TotalResponseTime.Nanoseconds()) / 1e6, // [ms]
		MaxResponseTime:      float64(entry.MaxResponseTime.Nanoseconds()) / 1e6,   // [ms]
		MinResponseTime:      minResponseTime,
		TotalContentLength:   entry.TotalContentLength,
		ResponseTimes:        entry.ResponseTimes,
		NumReqsPerSec:        entry.NumRequestsPerSec,
		NumFailPerSec:        entry.NumFailuresPerSec,
	}
}

func convertStatisticsError(key stats.ErrorKey, occurrence int64) *statsPayloadError {
	return &statsPayloadError{
		Name:        key.Name,
		Method:      key.Method,
		Error:       key.Error,
		Occurrences: occurrence,
	}
}
