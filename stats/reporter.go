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

type reportRequest struct {
	Now           time.Time
	RequestType   string
	Name          string
	ResponseTime  time.Duration
	ContentLength int64
	Error         error
}

type Reporter struct {
	ch chan<- reportRequest
}

func (r *Reporter) Init(statsCh chan<- *Stats) {
	r.ch = r.start(statsCh)
}

func (r *Reporter) Close() {
	close(r.ch)
}

func (r *Reporter) Report(requestType, name string, responseTime time.Duration, contentLength int64) {
	r.ch <- reportRequest{
		Now:           time.Now(),
		RequestType:   requestType,
		Name:          name,
		ResponseTime:  responseTime,
		ContentLength: contentLength,
		Error:         nil,
	}
}

func (r *Reporter) ReportError(requestType, name string, responseTime time.Duration, contentLength int64, err error) {
	r.ch <- reportRequest{
		Now:           time.Now(),
		RequestType:   requestType,
		Name:          name,
		ResponseTime:  responseTime,
		ContentLength: contentLength,
		Error:         err,
	}
}

func (r *Reporter) start(statsCh chan<- *Stats) chan<- reportRequest {
	s := New()
	ch := make(chan reportRequest, 100)

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case e, ok := <-ch:
				if !ok {
					statsCh <- s
					return
				}
				s.Add(e.Now, e.RequestType, e.Name, e.ResponseTime, e.ContentLength, e.Error)

			case <-ticker.C:
				statsCh <- s
				s = New()
			}
		}
	}()

	return ch
}
