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
	"bytes"
	"sync"
)

// LogCapture is a fixed-size ring buffer of the most recently written log lines.
// It implements io.Writer, so it can be plugged into whatever logger the
// caller already uses (the standard log package, slog, or anything else)
// by including it as one of the destinations of io.MultiWriter, e.g.:
//
//	capture := launce.NewLogCapture(500)
//	logger := slog.New(slog.NewTextHandler(io.MultiWriter(os.Stderr, capture), nil))
//
// This way the captured lines always match the exact format the caller
// configured, since it's the same handler writing to both destinations.
//
// It is used to report the worker's recent log lines to the master (see the
// "logs" message), so they can be inspected from the master without needing
// direct access to the worker host.
type LogCapture struct {
	mu    sync.Mutex
	lines []string
	start int
	count int
	total int64
	buf   []byte
}

// NewLogCapture returns a new LogCapture that keeps at most maxLines of the
// most recently written lines.
func NewLogCapture(maxLines int) *LogCapture {
	if maxLines < 0 {
		maxLines = 0
	}
	return &LogCapture{
		lines: make([]string, maxLines),
	}
}

// Write implements io.Writer. Input is split into lines, each of which is
// appended to the ring buffer. A trailing incomplete line is buffered until
// the rest of it is written.
func (c *LogCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buf = append(c.buf, p...)
	for {
		i := bytes.IndexByte(c.buf, '\n')
		if i < 0 {
			break
		}
		line := string(bytes.TrimRight(c.buf[:i], "\r"))
		c.append(line)
		c.buf = c.buf[i+1:]
	}

	return len(p), nil
}

func (c *LogCapture) append(line string) {
	c.total += 1

	maxLines := len(c.lines)
	if maxLines == 0 {
		return
	}

	idx := (c.start + c.count) % maxLines
	c.lines[idx] = line
	if c.count < maxLines {
		c.count += 1
	} else {
		c.start = (c.start + 1) % maxLines
	}
}

// Lines returns a snapshot of the currently buffered lines, oldest first.
func (c *LogCapture) Lines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	maxLines := len(c.lines)
	lines := make([]string, c.count)
	for i := 0; i < c.count; i++ {
		lines[i] = c.lines[(c.start+i)%maxLines]
	}
	return lines
}

// Total returns the cumulative number of lines ever recorded, including
// ones already evicted from the ring buffer. It is used to detect whether
// new lines have arrived since the buffer was last reported, even after
// the buffer has wrapped around.
func (c *LogCapture) Total() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}
