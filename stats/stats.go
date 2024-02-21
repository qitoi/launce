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
	Total   *Entry
	Errors  Errors
}

func New() *Stats {
	return &Stats{
		Entries: Entries{},
		Total:   newEntry(),
		Errors:  Errors{},
	}
}

func (s *Stats) Flush() (Entries, *Entry, Errors) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, total, errors := s.Entries, s.Total, s.Errors
	s.Entries = Entries{}
	s.Total = newEntry()
	s.Errors = Errors{}
	return entries, total, errors
}

func (s *Stats) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries = Entries{}
	s.Total = newEntry()
	s.Errors = Errors{}
}

func (s *Stats) Add(now time.Time, requestType, name string, opts Options) {
	key := EntryKey{requestType, name}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Total.Add(now, opts)

	if _, ok := s.Entries[key]; !ok {
		s.Entries[key] = newEntry()
	}
	s.Entries[key].Add(now, opts)

	if opts.Error != nil {
		s.Errors.Add(requestType, name, opts.Error)
	}
}

type Entries map[EntryKey]*Entry

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

func (s *Entry) Add(now time.Time, opt Options) {
	t := now.Unix()

	if s.StartTime == unixTimeZero || s.StartTime.After(now) {
		s.StartTime = now
	}
	if s.LastRequestTimestamp.Before(now) {
		s.LastRequestTimestamp = now
	}

	s.NumRequests += 1

	if opt.ResponseTime == nil {
		s.NumNoneRequests += 1
	} else {
		respTime := *opt.ResponseTime
		s.TotalResponseTime += respTime
		if s.MinResponseTime == nil {
			s.MinResponseTime = &respTime
		} else if *s.MinResponseTime > respTime {
			*s.MinResponseTime = respTime
		}
		s.MaxResponseTime = max(s.MaxResponseTime, respTime)
		rounded := roundResponseTime(respTime)
		if _, ok := s.ResponseTimes[rounded]; !ok {
			s.ResponseTimes[rounded] = 1
		} else {
			s.ResponseTimes[rounded] += 1
		}
	}

	s.TotalContentLength += opt.ResponseLength

	if _, ok := s.NumRequestsPerSec[t]; !ok {
		s.NumRequestsPerSec[t] = 0
	}
	s.NumRequestsPerSec[t] += 1

	if opt.Error != nil {
		s.NumFailures += 1
		s.NumFailuresPerSec[t] += 1
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

func (s Errors) Add(method, name string, err error) {
	key := ErrorKey{method, name, err.Error()}
	if _, ok := s[key]; !ok {
		s[key] = 0
	}
	s[key] += 1
}

type Options struct {
	ResponseTime   *time.Duration
	ResponseLength int64
	Error          error
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
