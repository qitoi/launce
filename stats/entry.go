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

package stats

import (
	"time"
)

// Entries represents collection of Entry for request types.
type Entries map[EntryKey]*Entry

// Merge merges statistics from src into this Entries
func (e *Entries) Merge(src Entries) {
	for k, v := range src {
		if _, ok := (*e)[k]; !ok {
			(*e)[k] = newEntry()
		}
		(*e)[k].Merge(v)
	}
}

// Aggregate aggregates all statistics for request types into one Entry.
func (e *Entries) Aggregate() *Entry {
	total := newEntry()
	for _, n := range *e {
		total.Merge(n)
	}
	return total
}

// EntryKey is request type.
type EntryKey struct {
	Method string
	Name   string
}

// Entry contains statistics for request.
type Entry struct {
	StartTime int64 // [ns]

	NumRequests          int64
	NumNoneRequests      int64
	NumRequestsPerSec    map[int64]int64
	NumFailures          int64
	NumFailuresPerSec    map[int64]int64
	LastRequestTimestamp int64 // [ns]

	TotalResponseTime time.Duration
	MinResponseTime   time.Duration
	MaxResponseTime   time.Duration

	TotalContentLength int64

	ResponseTimes map[int64]int64
}

// Add adds result of request to Entry.
func (e *Entry) Add(now time.Time, responseTime time.Duration, contentLength int64, err error) {
	t := now.Unix()
	nowNano := now.UnixNano()

	if e.StartTime == 0 || e.StartTime > nowNano {
		e.StartTime = nowNano
	}
	if e.LastRequestTimestamp < nowNano {
		e.LastRequestTimestamp = nowNano
	}

	e.NumRequests += 1

	if responseTime < 0 {
		e.NumNoneRequests += 1
	} else {
		e.TotalResponseTime += responseTime
		if e.MinResponseTime < 0 {
			e.MinResponseTime = responseTime
		} else if e.MinResponseTime < 0 || e.MinResponseTime > responseTime {
			e.MinResponseTime = responseTime
		}
		e.MaxResponseTime = max(e.MaxResponseTime, responseTime)
		rounded := roundResponseTime(responseTime)
		if _, ok := e.ResponseTimes[rounded]; !ok {
			e.ResponseTimes[rounded] = 1
		} else {
			e.ResponseTimes[rounded] += 1
		}
	}

	e.TotalContentLength += contentLength

	if _, ok := e.NumRequestsPerSec[t]; !ok {
		e.NumRequestsPerSec[t] = 1
	} else {
		e.NumRequestsPerSec[t] += 1
	}

	if err != nil {
		e.NumFailures += 1
		e.NumFailuresPerSec[t] += 1
	}
}

// Merge merges statistics from src into this Entry
func (e *Entry) Merge(src *Entry) {
	if e.StartTime == 0 || e.StartTime > src.StartTime {
		e.StartTime = src.StartTime
	}
	e.NumRequests += src.NumRequests
	e.NumNoneRequests += src.NumNoneRequests
	for k, v := range src.NumRequestsPerSec {
		if _, ok := e.NumRequestsPerSec[k]; !ok {
			e.NumRequestsPerSec[k] = v
		} else {
			e.NumRequestsPerSec[k] += v
		}
	}
	e.NumFailures += src.NumFailures
	for k, v := range src.NumFailuresPerSec {
		if _, ok := e.NumFailuresPerSec[k]; !ok {
			e.NumFailuresPerSec[k] = v
		} else {
			e.NumFailuresPerSec[k] += v
		}
	}
	if e.LastRequestTimestamp < src.LastRequestTimestamp {
		e.LastRequestTimestamp = src.LastRequestTimestamp
	}
	e.TotalResponseTime += src.TotalResponseTime
	if e.MinResponseTime < 0 || (src.MinResponseTime >= 0 && e.MinResponseTime > src.MinResponseTime) {
		e.MinResponseTime = src.MinResponseTime
	}
	if e.MaxResponseTime < src.MaxResponseTime {
		e.MaxResponseTime = src.MaxResponseTime
	}
	e.TotalContentLength += src.TotalContentLength
	for k, v := range src.ResponseTimes {
		if _, ok := e.ResponseTimes[k]; !ok {
			e.ResponseTimes[k] = v
		} else {
			e.ResponseTimes[k] += v
		}
	}
}

func newEntry() *Entry {
	return &Entry{
		StartTime:            0,
		NumRequests:          0,
		NumNoneRequests:      0,
		NumRequestsPerSec:    map[int64]int64{},
		NumFailures:          0,
		NumFailuresPerSec:    map[int64]int64{},
		LastRequestTimestamp: 0,
		TotalResponseTime:    0,
		MinResponseTime:      -1,
		MaxResponseTime:      0,
		ResponseTimes:        map[int64]int64{},
	}
}
