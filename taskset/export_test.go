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

type FilterOption = filterOption

func IncludeTags(tags ...string) FilterOption {
	return includeTags(tags...)
}

func ExcludeTags(tags ...string) FilterOption {
	return excludeTags(tags...)
}

func FilterTasks(tasks []Task, opts ...FilterOption) []Task {
	return filterTasks(tasks, opts...)
}

func Run(ctx context.Context, task Task, user launce.User) error {
	return run(ctx, task, user)
}
