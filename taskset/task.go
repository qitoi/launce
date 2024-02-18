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

package taskset

import (
	"context"

	"github.com/qitoi/launce"
)

var (
	_ Task = TaskFunc(nil)
)

type Task interface {
	Run(ctx context.Context, u launce.User, s Scheduler) error
}

type TaskFunc func(ctx context.Context, u launce.User, s Scheduler) error

func (t TaskFunc) Run(ctx context.Context, u launce.User, s Scheduler) error {
	return t(ctx, u, s)
}

type Scheduler interface {
	Schedule(task Task, first bool)
}
