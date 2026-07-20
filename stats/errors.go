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
	"time"
)

// Errors is an occurrence count of errors.
type Errors map[ErrorKey]ErrorOccurrence

// ErrorOccurrence contains the occurrence count and first/last seen time of an error.
type ErrorOccurrence struct {
	Count     int64
	FirstSeen int64 // [ns]
	LastSeen  int64 // [ns]
}

// Add adds an error to the occurrence count.
func (e *Errors) Add(now time.Time, method, name string, err error) {
	key := ErrorKey{method, name, err.Error()}
	// キーが無い場合は ErrorOccurrence のゼロ値が返るので、事前の存在チェックは不要
	o := (*e)[key]

	nowNano := now.UnixNano()
	if o.FirstSeen == 0 || o.FirstSeen > nowNano {
		o.FirstSeen = nowNano
	}
	if o.LastSeen < nowNano {
		o.LastSeen = nowNano
	}
	o.Count += 1

	(*e)[key] = o
}

// Merge merges the occurrence count of errors.
func (e *Errors) Merge(src Errors) {
	for k, v := range src {
		o := (*e)[k]

		if o.FirstSeen == 0 || (v.FirstSeen != 0 && o.FirstSeen > v.FirstSeen) {
			o.FirstSeen = v.FirstSeen
		}
		if o.LastSeen < v.LastSeen {
			o.LastSeen = v.LastSeen
		}
		o.Count += v.Count

		(*e)[k] = o
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
