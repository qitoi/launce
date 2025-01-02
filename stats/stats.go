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

// Package stats implements statistics for Locust.
package stats

import (
	"time"
)

// Stats represents statistics of request results.
type Stats struct {
	Entries Entries
	Errors  Errors

	startTime int64
}

// New returns a new Stats.
func New() *Stats {
	s := &Stats{}
	s.Clear()
	return s
}

// Flush returns the current statistics and clears the statistics.
func (s *Stats) Flush() (Entries, *Entry, Errors) {
	entries, errors := s.Entries, s.Errors
	total := entries.Aggregate()
	total.StartTime = s.startTime

	s.Clear()

	return entries, total, errors
}

// Clear clears the statistics.
func (s *Stats) Clear() {
	s.startTime = time.Now().UnixNano()
	s.Entries = Entries{}
	s.Errors = Errors{}
}

// Add adds a request to the statistics.
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

// Merge merges the statistics from src.
func (s *Stats) Merge(src *Stats) {
	s.Entries.Merge(src.Entries)
	s.Errors.Merge(src.Errors)
}
