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

// MessageHandler defines a function to handle custom messages from the master.
type MessageHandler func(msg Message)

// Runner is interface for load test runner.
type Runner interface {
	Host() string
	Tags() (tags, excludeTags *[]string)

	Options(v interface{}) error

	RegisterMessage(typ string, handler MessageHandler)
	SendMessage(typ string, data any) error

	ReportException(err error)
}
