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
	"time"
)

type WorkerOption func(w *Worker)

// WithVersion sets worker version.
func WithVersion(version string) WorkerOption {
	return func(w *Worker) {
		w.version = version
	}
}

// WithClientID sets client unique id.
func WithClientID(clientID string) WorkerOption {
	return func(w *Worker) {
		w.clientID = clientID
	}
}

// WithHeartbeatInterval sets the interval at which the worker sends a heartbeat message to the master.
func WithHeartbeatInterval(heartbeatInterval time.Duration) WorkerOption {
	return func(w *Worker) {
		w.heartbeatInterval = heartbeatInterval
	}
}

// WithMasterHeartbeatTimeout sets the timeout for the heartbeat from the master.
func WithMasterHeartbeatTimeout(masterHeartbeatTimeout time.Duration) WorkerOption {
	return func(w *Worker) {
		w.masterHeartbeatTimeout = masterHeartbeatTimeout
	}
}

// WithMetricsMonitorInterval sets the interval at which the worker monitors the process metrics.
func WithMetricsMonitorInterval(monitorInterval time.Duration) WorkerOption {
	return func(w *Worker) {
		w.metricsMonitorInterval = monitorInterval
	}
}

// WithStatsReportInterval sets the interval at which the worker sends statistics to the master.
func WithStatsReportInterval(reportInterval time.Duration) WorkerOption {
	return func(w *Worker) {
		w.statsReportInterval = reportInterval
	}
}

// WithStatsAggregationInterval sets the interval at which the worker aggregates statistics of users.
func WithStatsAggregationInterval(aggregateInterval time.Duration) WorkerOption {
	return func(w *Worker) {
		w.loadGenerator.StatsNotifyInterval = aggregateInterval
	}
}

// WithStatsAggregationUsers sets the number of users for which an Aggregator aggregates request statistics.
func WithStatsAggregationUsers(statsAggregationUsers int) WorkerOption {
	return func(w *Worker) {
		w.loadGenerator.StatsAggregationUsers = statsAggregationUsers
	}
}
