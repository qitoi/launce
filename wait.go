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
	"math/rand"
	"time"
)

var (
	nowFunc = time.Now
)

// WaitTimeFunc represents a wait time function for users or tasks.
type WaitTimeFunc func() time.Duration

// Between returns a WaitTimeFunc that returns a random wait time between min and max.
func Between(min, max time.Duration) WaitTimeFunc {
	return func() time.Duration {
		return time.Duration(rand.Int63n(max.Nanoseconds()-min.Nanoseconds()) + min.Nanoseconds())
	}
}

// Constant returns a WaitTimeFunc that returns the constant wait time.
func Constant(d time.Duration) WaitTimeFunc {
	return func() time.Duration {
		return d
	}
}

// ConstantPacing returns a WaitTimeFunc that calculates wait time to ensure the interval between task executions is d.
func ConstantPacing(d time.Duration) WaitTimeFunc {
	var lastRun = nowFunc()
	var lastWaitTime time.Duration

	return func() time.Duration {
		now := nowFunc()
		runTime := now.Sub(lastRun) - lastWaitTime
		lastWaitTime = max(0, d-runTime)
		lastRun = now
		if lastWaitTime <= 0 {
			return 0
		}
		return lastWaitTime
	}
}

// ConstantThroughput returns a WaitTimeFunc that returns wait time that makes the number of task executions per second equal to taskRunsPerSecond.
func ConstantThroughput(taskRunsPerSecond float64) WaitTimeFunc {
	return ConstantPacing(time.Duration(float64(time.Second) / taskRunsPerSecond))
}
