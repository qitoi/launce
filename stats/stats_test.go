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

package stats_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/qitoi/launce/stats"
)

func ptr[T any](v T) *T {
	return &v
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return t
}

func log(s *stats.Stats, typ, name, datetime string, duration int, size int64, err error) {
	tm := parseTime(datetime)
	s.Add(tm, typ, name, size, duration, err)
}

func getStats() *stats.Stats {
	s := stats.New()

	log(s, "GET", "/test1", "2024-01-01T00:00:00.110Z", -1, -1, nil)
	log(s, "GET", "/test2", "2024-01-01T00:00:00.080Z", 8, 10, nil)
	log(s, "GET", "/test1", "2024-01-01T00:00:00.100Z", 116, 60, nil)
	log(s, "GET", "/test2", "2024-01-01T00:00:01.200Z", 58, -1, nil)
	log(s, "GET", "/test3", "2024-01-01T00:00:01.600Z", -1, 0, nil)
	log(s, "GET", "/error", "2024-01-01T00:00:01.800Z", 1234, 0, errors.New("error"))
	log(s, "GET", "/error", "2024-01-01T00:00:01.900Z", 121, 128, errors.New("error2"))
	log(s, "GET", "/test1", "2024-01-01T00:00:01.999999999Z", 933, 180, nil)
	log(s, "GET", "/test1", "2024-01-01T00:00:01.300Z", 124, 888, errors.New("error"))
	log(s, "GET", "/error", "2024-01-01T00:00:02.500Z", 10777, 2048, errors.New("error"))

	return s
}

func extractEntriesField[T any](s *stats.Stats, f func(e *stats.Entry) T) map[string]T {
	fields := map[string]T{}
	for key, entry := range s.Entries {
		fields[fmt.Sprintf("%s:%s", key.Method, key.Name)] = f(entry)
	}
	return fields
}

func TestStatistics_Entries_StartTime(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) time.Time {
		return e.StartTime
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]time.Time{
		"GET:/test1": parseTime("2024-01-01T00:00:00.100Z"),
		"GET:/test2": parseTime("2024-01-01T00:00:00.080Z"),
		"GET:/test3": parseTime("2024-01-01T00:00:01.600Z"),
		"GET:/error": parseTime("2024-01-01T00:00:01.800Z"),
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]time.Time{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_NumRequests(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) int64 {
		return e.NumRequests
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]int64{
		"GET:/test1": 4,
		"GET:/test2": 2,
		"GET:/test3": 1,
		"GET:/error": 3,
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]int64{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_NumNoneRequests(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) int64 {
		return e.NumNoneRequests
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]int64{
		"GET:/test1": 1,
		"GET:/test2": 0,
		"GET:/test3": 1,
		"GET:/error": 0,
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]int64{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_NumRequestsPerSec(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) map[int64]int64 {
		return e.NumRequestsPerSec
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]map[int64]int64{
		"GET:/test1": {
			1704067200: 2,
			1704067201: 2,
		},
		"GET:/test2": {
			1704067200: 1,
			1704067201: 1,
		},
		"GET:/test3": {
			1704067201: 1,
		},
		"GET:/error": {
			1704067201: 2,
			1704067202: 1,
		},
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]map[int64]int64{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_NumFailures(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) int64 {
		return e.NumFailures
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]int64{
		"GET:/test1": 1,
		"GET:/test2": 0,
		"GET:/test3": 0,
		"GET:/error": 3,
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]int64{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_NumFailuresPerSec(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) map[int64]int64 {
		return e.NumFailuresPerSec
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]map[int64]int64{
		"GET:/test1": {
			1704067201: 1,
		},
		"GET:/test2": {},
		"GET:/test3": {},
		"GET:/error": {
			1704067201: 2,
			1704067202: 1,
		},
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]map[int64]int64{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_LastRequestTimestamp(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) time.Time {
		return e.LastRequestTimestamp
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]time.Time{
		"GET:/test1": parseTime("2024-01-01T00:00:01.999999999Z"),
		"GET:/test2": parseTime("2024-01-01T00:00:01.200Z"),
		"GET:/test3": parseTime("2024-01-01T00:00:01.600Z"),
		"GET:/error": parseTime("2024-01-01T00:00:02.500Z"),
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]time.Time{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_TotalResponseTime(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) time.Duration {
		return e.TotalResponseTime
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]time.Duration{
		"GET:/test1": 1173 * time.Millisecond,
		"GET:/test2": 66 * time.Millisecond,
		"GET:/test3": 0 * time.Millisecond,
		"GET:/error": 12132 * time.Millisecond,
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]time.Duration{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_MinResponseTime(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) *time.Duration {
		return e.MinResponseTime
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]*time.Duration{
		"GET:/test1": ptr(116 * time.Millisecond),
		"GET:/test2": ptr(8 * time.Millisecond),
		"GET:/test3": nil,
		"GET:/error": ptr(121 * time.Millisecond),
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]*time.Duration{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_MaxResponseTime(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) time.Duration {
		return e.MaxResponseTime
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]time.Duration{
		"GET:/test1": 933 * time.Millisecond,
		"GET:/test2": 58 * time.Millisecond,
		"GET:/test3": 0,
		"GET:/error": 10777 * time.Millisecond,
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]time.Duration{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_TotalContentLength(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) int64 {
		return e.TotalContentLength
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]int64{
		"GET:/test1": 1128,
		"GET:/test2": 10,
		"GET:/test3": 0,
		"GET:/error": 2176,
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]int64{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Entries_ResponseTimes(t *testing.T) {
	s := getStats()
	extractor := func(e *stats.Entry) map[int64]int64 {
		return e.ResponseTimes
	}

	fields := extractEntriesField(s, extractor)
	expected := map[string]map[int64]int64{
		"GET:/test1": {
			120: 2,
			930: 1,
		},
		"GET:/test2": {
			8:  1,
			58: 1,
		},
		"GET:/test3": {},
		"GET:/error": {
			120:   1,
			1200:  1,
			11000: 1,
		},
	}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}

	s.Flush()

	fields = extractEntriesField(s, extractor)
	expected = map[string]map[int64]int64{}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", fields, expected)
	}
}

func TestStatistics_Errors(t *testing.T) {
	s := getStats()

	expected := stats.Errors{
		{"GET", "/test1", "error"}:  1,
		{"GET", "/error", "error"}:  2,
		{"GET", "/error", "error2"}: 1,
	}
	if !reflect.DeepEqual(s.Errors, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", s.Errors, expected)
	}

	s.Flush()

	expected = stats.Errors{}
	if !reflect.DeepEqual(s.Errors, expected) {
		t.Fatalf("invalid value. got:%v, want:%v", s.Errors, expected)
	}
}
