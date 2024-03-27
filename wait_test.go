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
	"fmt"
	"testing"
	"time"
)

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestConstant(t *testing.T) {
	testcases := []time.Duration{
		1 * time.Millisecond,
		22 * time.Second,
		333 * time.Hour,
	}
	for n, testcase := range testcases {
		t.Run(fmt.Sprintf("case #%d", n+1), func(t *testing.T) {
			actual := Constant(testcase)()
			if testcase != actual {
				t.Fatalf("invalid duration got:%v, want:%v", actual, testcase)
			}
		})
	}
}

func TestBetween(t *testing.T) {
	testcases := []struct {
		Min time.Duration
		Max time.Duration
	}{
		{
			1 * time.Millisecond,
			10 * time.Millisecond,
		},
		{
			1 * time.Nanosecond,
			2 * time.Nanosecond,
		},
		{
			1 * time.Millisecond,
			1 * time.Second,
		},
	}
	for n, testcase := range testcases {
		t.Run(fmt.Sprintf("case #%d", n+1), func(t *testing.T) {
			for i := 0; i < 1000; i++ {
				actual := Between(testcase.Min, testcase.Max)()
				if actual < testcase.Min && testcase.Max < actual {
					t.Fatalf("invalid duration got:%v, min:%v, max:%v", actual, testcase.Min, testcase.Max)
				}
			}
		})
	}
}

func TestConstantPacing(t *testing.T) {
	type step struct {
		TIme     time.Time
		Expected time.Duration
	}
	testcases := []struct {
		StartTime time.Time
		Duration  time.Duration
		Steps     []step
	}{
		{
			StartTime: parseTime("2024-01-01T00:00:00Z"),
			Duration:  5 * time.Second,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:00Z"),
					5 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:05Z"),
					5 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:10Z"),
					5 * time.Second,
				},
			},
		},
		{
			StartTime: parseTime("2024-01-01T00:00:00Z"),
			Duration:  5 * time.Second,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:01Z"),
					4 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:06Z"),
					4 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:11Z"),
					4 * time.Second,
				},
			},
		},
		{
			StartTime: parseTime("2024-01-01T00:00:00Z"),
			Duration:  5 * time.Second,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:08Z"),
					0 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:09Z"),
					4 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:14Z"),
					4 * time.Second,
				},
			},
		},
		{
			StartTime: parseTime("2024-01-01T00:00:00Z"),
			Duration:  5 * time.Second,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:08Z"),
					0 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:09Z"),
					4 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:19Z"),
					0 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:20Z"),
					4 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:24Z"),
					5 * time.Second,
				},
			},
		},
		{
			StartTime: parseTime("2024-01-01T00:00:00Z"),
			Duration:  5 * time.Second,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:01Z"),
					4 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:02Z"),
					8 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:03Z"),
					12 * time.Second,
				},
			},
		},
		{
			StartTime: parseTime("2024-01-01T00:00:00Z"),
			Duration:  5 * time.Second,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:01Z"),
					4 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:02Z"),
					8 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:11Z"),
					4 * time.Second,
				},
			},
		},
	}
	for n, testcase := range testcases {
		t.Run(fmt.Sprintf("case #%d", n+1), func(t *testing.T) {
			nowFunc = func() time.Time {
				return testcase.StartTime
			}
			waitTimeFunc := ConstantPacing(testcase.Duration)
			for _, step := range testcase.Steps {
				nowFunc = func() time.Time {
					return step.TIme
				}
				d := waitTimeFunc()
				if d != step.Expected {
					t.Errorf("invalid duration got:%v want:%v", d, step.Expected)
				}
			}
		})
	}
}

func TestConstantThroughput(t *testing.T) {
	type step struct {
		TIme     time.Time
		Expected time.Duration
	}
	testcases := []struct {
		StartTime  time.Time
		Throughput float64
		Steps      []step
	}{
		{
			StartTime:  parseTime("2024-01-01T00:00:00Z"),
			Throughput: 1,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:00Z"),
					1 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:01Z"),
					1 * time.Second,
				},
				{
					parseTime("2024-01-01T00:00:02Z"),
					1 * time.Second,
				},
			},
		},
		{
			StartTime:  parseTime("2024-01-01T00:00:00Z"),
			Throughput: 2,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:00Z"),
					500 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:00.9Z"),
					100 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:01.3Z"),
					200 * time.Millisecond,
				},
			},
		},
		{
			StartTime:  parseTime("2024-01-01T00:00:00Z"),
			Throughput: 2,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:01.2Z"),
					0 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:01.3Z"),
					400 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:01.7Z"),
					500 * time.Millisecond,
				},
			},
		},
		{
			StartTime:  parseTime("2024-01-01T00:00:00Z"),
			Throughput: 2,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:01.2Z"),
					0 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:01.3Z"),
					400 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:01.7Z"),
					500 * time.Millisecond,
				},
			},
		},
		{
			StartTime:  parseTime("2024-01-01T00:00:00Z"),
			Throughput: 2,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:00.1Z"),
					400 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:00.2Z"),
					800 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:00.3Z"),
					1200 * time.Millisecond,
				},
			},
		},
		{
			StartTime:  parseTime("2024-01-01T00:00:00Z"),
			Throughput: 2.5,
			Steps: []step{
				{
					parseTime("2024-01-01T00:00:00Z"),
					400 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:00.4Z"),
					400 * time.Millisecond,
				},
				{
					parseTime("2024-01-01T00:00:00.8Z"),
					400 * time.Millisecond,
				},
			},
		},
	}
	for n, testcase := range testcases {
		t.Run(fmt.Sprintf("case #%d", n+1), func(t *testing.T) {
			nowFunc = func() time.Time {
				return testcase.StartTime
			}
			waitTimeFunc := ConstantThroughput(testcase.Throughput)
			for _, step := range testcase.Steps {
				nowFunc = func() time.Time {
					return step.TIme
				}
				d := waitTimeFunc()
				if d != step.Expected {
					t.Errorf("invalid duration got:%v want:%v", d, step.Expected)
				}
			}
		})
	}
}
