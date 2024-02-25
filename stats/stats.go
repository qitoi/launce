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
	"crypto/sha256"
	"fmt"
	"math"
	"sync"
	"time"
)

var (
	unixTimeZero = time.Unix(0, 0)
)

type Stats struct {
	mu sync.Mutex

	Entries Entries
	Errors  Errors
}

func New() *Stats {
	return &Stats{
		Entries: Entries{},
		Errors:  Errors{},
	}
}

func (s *Stats) Flush() (Entries, *Entry, Errors) {
	entries, errors := s.Entries, s.Errors
	s.Entries = Entries{}
	s.Errors = Errors{}
	total := entries.Aggregate()
	return entries, total, errors
}

func (s *Stats) Clear() {
	s.Entries = Entries{}
	s.Errors = Errors{}
}

func (s *Stats) Add(now time.Time, requestType, name string, responseTime time.Duration, contentLength int64, err error) {
	var key = EntryKey{
		Method: requestType,
		Name:   name,
	}
	if _, ok := s.Entries[key]; !ok {
		s.Entries[key] = newEntry()
	}
	s.Entries[key].Add(now, responseTime, contentLength, err)

	if err != nil {
		s.Errors.Add(key.Method, key.Name, err)
	}
}

func (s *Stats) Merge(src *Stats) {
	s.Entries.Merge(src.Entries)
	s.Errors.Merge(src.Errors)
}

type Entries map[EntryKey]*Entry

func (e *Entries) Merge(src Entries) {
	for k, v := range src {
		if _, ok := (*e)[k]; !ok {
			(*e)[k] = newEntry()
		}
		(*e)[k].Merge(v)
	}
}

func (e *Entries) Aggregate() *Entry {
	total := newEntry()
	for _, n := range *e {
		total.Merge(n)
	}
	return total
}

type EntryKey struct {
	Method string
	Name   string
}

type Entry struct {
	StartTime time.Time

	NumRequests          int64
	NumNoneRequests      int64
	NumRequestsPerSec    map[int64]int64
	NumFailures          int64
	NumFailuresPerSec    map[int64]int64
	LastRequestTimestamp time.Time

	TotalResponseTime time.Duration
	MinResponseTime   *time.Duration
	MaxResponseTime   time.Duration

	TotalContentLength int64

	ResponseTimes map[int64]int64
}

func (e *Entry) Add(now time.Time, responseTime time.Duration, contentLength int64, err error) {
	t := now.Unix()

	if e.StartTime == unixTimeZero || e.StartTime.After(now) {
		e.StartTime = now
	}
	if e.LastRequestTimestamp.Before(now) {
		e.LastRequestTimestamp = now
	}

	e.NumRequests += 1

	if responseTime < 0 {
		e.NumNoneRequests += 1
	} else {
		e.TotalResponseTime += responseTime
		if e.MinResponseTime == nil {
			e.MinResponseTime = &responseTime
		} else if *e.MinResponseTime > responseTime {
			*e.MinResponseTime = responseTime
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
		e.NumRequestsPerSec[t] = 0
	}
	e.NumRequestsPerSec[t] += 1

	if err != nil {
		e.NumFailures += 1
		e.NumFailuresPerSec[t] += 1
	}
}

func (e *Entry) Merge(src *Entry) {
	if e.StartTime == unixTimeZero || e.StartTime.After(src.StartTime) {
		e.StartTime = src.StartTime
	}
	e.NumRequests += src.NumRequests
	e.NumNoneRequests += src.NumNoneRequests
	for k, v := range src.NumRequestsPerSec {
		if _, ok := e.NumRequestsPerSec[k]; !ok {
			e.NumRequestsPerSec[k] = 0
		}
		e.NumRequestsPerSec[k] += v
	}
	e.NumFailures += src.NumFailures
	for k, v := range src.NumFailuresPerSec {
		if _, ok := e.NumFailuresPerSec[k]; !ok {
			e.NumFailuresPerSec[k] = 0
		}
		e.NumFailuresPerSec[k] += v
	}
	if e.LastRequestTimestamp.Before(src.LastRequestTimestamp) {
		e.LastRequestTimestamp = src.LastRequestTimestamp
	}
	e.TotalResponseTime += src.TotalResponseTime
	if e.MinResponseTime == nil {
		var d time.Duration
		e.MinResponseTime = &d
	}
	if src.MinResponseTime != nil && *e.MinResponseTime > *src.MinResponseTime {
		*e.MinResponseTime = *src.MinResponseTime
	}
	if e.MaxResponseTime < src.MaxResponseTime {
		e.MaxResponseTime = src.MaxResponseTime
	}
	e.TotalContentLength += src.TotalContentLength
	for k, v := range src.ResponseTimes {
		if _, ok := e.ResponseTimes[k]; !ok {
			e.ResponseTimes[k] = 0
		}
		e.ResponseTimes[k] += v
	}
}

func newEntry() *Entry {
	return &Entry{
		StartTime:            unixTimeZero,
		NumRequests:          0,
		NumNoneRequests:      0,
		NumRequestsPerSec:    map[int64]int64{},
		NumFailures:          0,
		NumFailuresPerSec:    map[int64]int64{},
		LastRequestTimestamp: unixTimeZero,
		TotalResponseTime:    0,
		MinResponseTime:      nil,
		MaxResponseTime:      0,
		ResponseTimes:        map[int64]int64{},
	}
}

type ErrorKey struct {
	Method string
	Name   string
	Error  string
}

func (s *ErrorKey) Encode() string {
	d := sha256.New()
	d.Write([]byte(s.Method + "." + s.Name + "." + s.Error))
	return fmt.Sprintf("%x", d.Sum(nil))
}

type Errors map[ErrorKey]int64

func (e *Errors) Add(method, name string, err error) {
	key := ErrorKey{method, name, err.Error()}
	if _, ok := (*e)[key]; !ok {
		(*e)[key] = 0
	}
	(*e)[key] += 1
}

func (e *Errors) Merge(src Errors) {
	for k, v := range src {
		if _, ok := (*e)[k]; !ok {
			(*e)[k] = 0
		}
		(*e)[k] += v
	}
}

func roundResponseTime(d time.Duration) int64 {
	s := float64(d.Microseconds()) / 1e3
	switch {
	case s < 100:
		return int64(math.Round(s))
	case s < 1000:
		return int64(math.Round(s/10)) * 10
	case s < 10000:
		return int64(math.Round(s/100)) * 100
	default:
		return int64(math.Round(s/1000)) * 1000
	}
}
