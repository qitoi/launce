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

package main

import (
	"context"
	"net/http"
	"time"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/taskset"
)

/*
 * Run locust --master --tags tag1:
     task1 and task2 would be executed

 * Run locust --master --tags tag2 tag3:
     task2 and task3 would be executed

 * Run locust --master --exclude-tags tag3:
     task1, task2, and task4 would be executed

 * Run locust --master --tags tag1 tag2 --exclude-tags tag2:
     task1 would be executed
*/

var (
	_ taskset.UserRequirement = (*User)(nil)
)

type User struct {
	taskset.User
}

func (u *User) WaitTime() launce.WaitTimeFunc {
	return launce.Between(100*time.Millisecond, 200*time.Millisecond)
}

func (u *User) TaskSet() taskset.TaskSet {
	return taskset.NewSequential(
		taskset.Tag(taskset.TaskFunc(u.task1), "tag1"),
		taskset.Tag(taskset.TaskFunc(u.task2), "tag1", "tag2"),
		taskset.Tag(taskset.TaskFunc(u.task3), "tag3"),
		taskset.TaskFunc(u.task4),
	)
}

func (u *User) task1(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	u.Report(
		http.MethodGet,
		"/task/1",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return nil
}

func (u *User) task2(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	u.Report(
		http.MethodGet,
		"/task/2",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return nil
}

func (u *User) task3(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	u.Report(
		http.MethodGet,
		"/task/3",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return nil
}

func (u *User) task4(_ context.Context, _ launce.User, _ taskset.Scheduler) error {
	u.Report(
		http.MethodGet,
		"/task/4",
		launce.WithResponseTime(100*time.Millisecond),
	)
	return nil
}
