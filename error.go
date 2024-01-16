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
	"runtime"
)

type Error struct {
	err      error
	file     string
	line     int
	function string
}

func (e Error) Error() string {
	return e.err.Error()
}

func (e Error) Unwrap() error {
	return e.err
}

func (e Error) Traceback() string {
	return fmt.Sprintf("%s:%d (%s)", e.file, e.line, e.function)
}

func Wrap(err error) error {
	pc := make([]uintptr, 1)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return &Error{
		err:      err,
		file:     frame.File,
		line:     frame.Line,
		function: frame.Function,
	}
}
