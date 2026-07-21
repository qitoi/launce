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

package launce_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/qitoi/launce"
)

func TestLogCapture_Write(t *testing.T) {
	c := launce.NewLogCapture(10)

	_, _ = fmt.Fprintf(c, "line1\nline2\n")

	got := c.Lines()
	want := []string{"line1", "line2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("invalid value. got:%v want:%v", got, want)
	}
	if total := c.Total(); total != 2 {
		t.Fatalf("invalid total. got:%d want:%d", total, 2)
	}
}

func TestLogCapture_Write_PartialLine(t *testing.T) {
	c := launce.NewLogCapture(10)

	_, _ = fmt.Fprint(c, "abc")
	if got := c.Lines(); len(got) != 0 {
		t.Fatalf("invalid value. got:%v want empty", got)
	}

	_, _ = fmt.Fprint(c, "def\n")
	got := c.Lines()
	want := []string{"abcdef"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("invalid value. got:%v want:%v", got, want)
	}
}

func TestLogCapture_Write_CRLF(t *testing.T) {
	c := launce.NewLogCapture(10)

	_, _ = fmt.Fprint(c, "line1\r\nline2\r\n")

	got := c.Lines()
	want := []string{"line1", "line2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("invalid value. got:%v want:%v", got, want)
	}
}

func TestLogCapture_RingBuffer_Wraparound(t *testing.T) {
	c := launce.NewLogCapture(3)

	for i := 0; i < 5; i++ {
		_, _ = fmt.Fprintf(c, "line%d\n", i)
	}

	got := c.Lines()
	want := []string{"line2", "line3", "line4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("invalid value. got:%v want:%v", got, want)
	}
	if total := c.Total(); total != 5 {
		t.Fatalf("invalid total. got:%d want:%d", total, 5)
	}
}

func TestLogCapture_MaxZero(t *testing.T) {
	c := launce.NewLogCapture(0)

	_, _ = fmt.Fprint(c, "line1\n")

	if got := c.Lines(); len(got) != 0 {
		t.Fatalf("invalid value. got:%v want empty", got)
	}
	if total := c.Total(); total != 1 {
		t.Fatalf("invalid total. got:%d want:%d", total, 1)
	}
}

func TestLogCapture_MultiWriter_WithSlog(t *testing.T) {
	c := launce.NewLogCapture(10)

	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(&out, c), nil)).With("req_id", "abc")
	logger.Info("hello", "key", "value")

	// capture 側と out 側は同じ Handler が書き込むので、内容は完全に一致するはず
	lines := c.Lines()
	if len(lines) != 1 {
		t.Fatalf("invalid number of captured lines. got:%d want:%d", len(lines), 1)
	}
	if got, want := lines[0]+"\n", out.String(); got != want {
		t.Fatalf("captured line does not match the other destination. got:%q want:%q", got, want)
	}
	if !strings.Contains(lines[0], "hello") || !strings.Contains(lines[0], "key=value") || !strings.Contains(lines[0], "req_id=abc") {
		t.Fatalf("captured line missing expected content. got:%q", lines[0])
	}
}
