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
	"runtime"
	"strconv"
	"strings"
)

type wrapError struct {
	err        error
	stackTrace string
}

func (e wrapError) Error() string {
	return e.err.Error()
}

func (e wrapError) Unwrap() error {
	return e.err
}

func (e wrapError) StackTrace() string {
	return e.stackTrace
}

// Wrap returns error with stack trace.
func Wrap(err error) error {
	builder := strings.Builder{}

	var pc [20]uintptr
	n := runtime.Callers(2, pc[:])
	frames := runtime.CallersFrames(pc[:n])

	for frame, ok := frames.Next(); ok; frame, ok = frames.Next() {
		builder.WriteString(frame.Function)
		builder.WriteString("\n    ")
		builder.WriteString(frame.File)
		builder.WriteString(":")
		builder.WriteString(strconv.Itoa(frame.Line))
		builder.WriteString("\n")
	}

	return wrapError{
		err:        err,
		stackTrace: builder.String(),
	}
}
