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

// Package launce is Locust worker library written in Go.
// The aim of this library is to write load test scenarios as simply as locustfile.py
// and to run load testing with better performance.
package launce

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
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

var (
	_ Runner = (*Worker)(nil)
)

// Worker provides the functionality of a Locust worker.
type Worker struct {
	// Version is the version of the worker
	version string

	// clientID is the unique identifier of the worker.
	clientID string

	// heartbeatInterval is the interval at which the worker sends a heartbeat message to the master.
	heartbeatInterval time.Duration

	// masterHeartbeatTimeout is the timeout for the heartbeat from the master.
	masterHeartbeatTimeout time.Duration

	// metricsMonitorInterval is the interval at which the worker monitors the process metrics.
	metricsMonitorInterval time.Duration

	// statsReportInterval is the interval at which the worker sends statistics to the master.
	statsReportInterval time.Duration

	catchExceptions bool

	loadGenerator *LoadGenerator
	index         int64
	state         WorkerState
	transport     Transport
	stats         *stats.Stats
	spawnCh       chan map[string]int64
	ackCh         chan struct{}
	heartbeatCh   chan struct{}
	procInfo      *processInfo

	host          atomic.Value
	parsedOptions atomic.Pointer[ParsedOptions]

	messageHandlers  map[string][]MessageHandler
	connectHandlers  handlers
	quittingHandlers handlers
	quitHandlers     handlers

	// cancel is the context cancel function of the join process.
	cancel atomic.Value
}

// NewWorker creates an instance of Worker.
func NewWorker(transport Transport, options ...WorkerOption) (*Worker, error) {
	id, err := generateClientID()
	if err != nil {
		return nil, err
	}
	procInfo, err := newProcessInfo(os.Getpid())
	if err != nil {
		return nil, err
	}

	w := &Worker{
		version:                fmt.Sprintf("%s.launce-%s", LocustVersion, Version),
		clientID:               id,
		heartbeatInterval:      defaultHeartbeatInterval,
		metricsMonitorInterval: defaultMetricsMonitorInterval,
		masterHeartbeatTimeout: defaultMasterHeartbeatTimeout,
		statsReportInterval:    defaultStatsReportInterval,
		catchExceptions:        true,

		loadGenerator: NewLoadGenerator(),
		index:         -1,
		state:         WorkerStateInit,
		transport:     transport,
		stats:         stats.New(),
		spawnCh:       make(chan map[string]int64),
		ackCh:         make(chan struct{}, 1),
		heartbeatCh:   make(chan struct{}),
		procInfo:      procInfo,

		messageHandlers: map[string][]MessageHandler{},
	}

	w.cancel.Store(context.CancelFunc(nil))

	for _, option := range options {
		option(w)
	}

	return w, nil
}

// Version returns the version of the worker
func (w *Worker) Version() string {
	return w.version
}

// ClientID returns the unique identifier of the worker
func (w *Worker) ClientID() string {
	return w.clientID
}

// Join connects to the master and starts the worker processes.
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

	w.startHeartbeatProcess(ctx, &procWg, w.heartbeatInterval)
	w.startHeartbeatCheckProcess(ctx, &procWg, w.masterHeartbeatTimeout)
	w.startMetricsMonitorProcess(ctx, &procWg, w.metricsMonitorInterval)
	w.startStatsProcess(ctx, &procWg, w.statsReportInterval)
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

	w.quitHandlers.Call()

	return nil
}

// Stop stops the load test.
func (w *Worker) Stop() {
	if s := atomic.LoadInt64(&w.state); s == WorkerStateCleanup || s == WorkerStateStopped {
		return
	}
	atomic.StoreInt64(&w.state, WorkerStateCleanup)
	w.loadGenerator.Stop()
	atomic.StoreInt64(&w.state, WorkerStateStopped)
}

// Quit stops the worker.
func (w *Worker) Quit() {
	cancel := w.cancel.Swap(context.CancelFunc(nil)).(context.CancelFunc)
	if cancel == nil {
		return
	}

	w.quittingHandlers.Call()
	w.Stop()
	cancel()
}

// RegisterUser registers user.
func (w *Worker) RegisterUser(name string, f func() User) {
	w.loadGenerator.RegisterUser(w, name, f)
}

// OnConnect registers function to be called when the worker connects to the master.
func (w *Worker) OnConnect(f func()) {
	w.connectHandlers.Add(f)
}

// OnQuitting registers function to be called when the worker starts quitting.
func (w *Worker) OnQuitting(f func()) {
	w.quittingHandlers.Add(f)
}

// OnQuit registers function to be called when the worker quits.
func (w *Worker) OnQuit(f func()) {
	w.quitHandlers.Add(f)
}

// OnTestStart registers function to be called when the test starts.
func (w *Worker) OnTestStart(f func(ctx context.Context) error) {
	w.loadGenerator.OnTestStart(f)
}

// OnTestStopping registers function to be called when the load test start stopping.
func (w *Worker) OnTestStopping(f func(ctx context.Context)) {
	w.loadGenerator.OnTestStopping(f)
}

// OnTestStop registers function to be called when the test stops.
func (w *Worker) OnTestStop(f func(ctx context.Context)) {
	w.loadGenerator.OnTestStop(f)
}

// Index returns the index of the worker.
func (w *Worker) Index() int64 {
	return atomic.LoadInt64(&w.index)
}

// State returns the state of the worker.
func (w *Worker) State() WorkerState {
	return atomic.LoadInt64(&w.state)
}

func (w *Worker) Host() string {
	if h := w.host.Load(); h != nil {
		if host, ok := h.(string); ok {
			return host
		}
	}
	return ""
}

func (w *Worker) Tags() (tags, excludeTags *[]string) {
	if p := w.parsedOptions.Load(); p != nil {
		return p.Tags, p.ExcludeTags
	}
	return nil, nil
}

func (w *Worker) Options(v interface{}) error {
	if p := w.parsedOptions.Load(); p != nil {
		return p.Extract(v)
	}
	return nil
}

func (w *Worker) ReportException(err error) {
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

func (w *Worker) CatchExceptions() bool {
	return w.catchExceptions
}

// RegisterMessage registers custom message handler.
func (w *Worker) RegisterMessage(typ string, handler MessageHandler) {
	w.messageHandlers[typ] = append(w.messageHandlers[typ], handler)
}

// SendMessage sends message to the master.
func (w *Worker) SendMessage(typ string, data any) error {
	msg := message{
		Type:   typ,
		Data:   data,
		NodeID: w.clientID,
	}
	b, err := encodeMessage(msg)
	if err != nil {
		return err
	}
	return w.transport.Send(b)
}

func (w *Worker) open(ctx context.Context) error {
	return w.transport.Open(ctx, w.clientID)
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

	if err := w.SendMessage(messageClientReady, w.version); err != nil {
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

	w.connectHandlers.Call()

	return nil
}

func (w *Worker) recv() (Message, error) {
	b, err := w.transport.Receive()
	if err != nil {
		return Message{}, err
	}
	return decodeMessage(b)
}

// startMessageProcess starts the process of receiving and processing messages from the master.
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

				w.host.Store(payload.Host)
				w.parsedOptions.Store(&payload.ParsedOptions)

				w.spawnCh <- payload.UserClassesCount

			case messageStop:
				w.loadGenerator.Stop()
				w.host.Store("")
				w.parsedOptions.Store(nil)
				_ = w.SendMessage(messageClientStopped, nil)
				atomic.StoreInt64(&w.state, WorkerStateInit)
				_ = w.SendMessage(messageClientReady, w.version)

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
				if handlers, ok := w.messageHandlers[msg.Type]; ok {
					for _, handler := range handlers {
						handler(msg)
					}
				}
			}
		}
	}()
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
					if err := w.loadGenerator.Start(); err != nil {
						continue
					}
				}

				atomic.StoreInt64(&w.state, WorkerStateSpawning)
				_ = w.SendMessage(messageSpawning, nil)

				users := w.loadGenerator.Users()
				var total int64
				for _, u := range users {
					total += u
				}

				payload := &spawningCompletePayload{
					UserClassesCount: make(map[string]int64),
					UserCount:        0,
				}
				for name, count := range spawnCount {
					if err := w.loadGenerator.Spawn(name, int(count)); err == nil {
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

	ch := w.loadGenerator.Stats()

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
	payload.UserClassesCount = w.loadGenerator.Users()
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
	if entry.MinResponseTime >= 0 {
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

type handlers []func()

func (h *handlers) Add(f func()) {
	*h = append(*h, f)
}

func (h *handlers) Call() {
	for _, f := range *h {
		f()
	}
}

func generateClientID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	var b [32]byte
	hex.Encode(b[:], id[:])
	return hostname + "_" + string(b[:]), nil
}
