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
)

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
