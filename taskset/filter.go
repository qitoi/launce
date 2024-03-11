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

type filterOptions struct {
	tags        *[]string
	excludeTags []string
}

type filterOption func(opt *filterOptions)

func includeTags(tags ...string) filterOption {
	return func(opt *filterOptions) {
		opt.tags = &tags
	}
}

func excludeTags(tags ...string) filterOption {
	return func(opt *filterOptions) {
		opt.excludeTags = tags
	}
}

func filterTasks(tasks []Task, opts ...filterOption) []Task {
	var option filterOptions
	for _, opt := range opts {
		opt(&option)
	}

	var tagMap map[string]struct{}
	if option.tags != nil {
		tagMap = make(map[string]struct{}, len(*option.tags))
		for _, tag := range *option.tags {
			tagMap[tag] = struct{}{}
		}
	}

	excludeTagMap := make(map[string]struct{})
	for _, tag := range option.excludeTags {
		excludeTagMap[tag] = struct{}{}
	}

	filtered := make([]Task, 0, len(tasks))
loop:
	for _, task := range tasks {
		taskTags := GetTags(task)

		for _, tag := range taskTags {
			if _, ok := excludeTagMap[tag]; ok {
				continue loop
			}
		}

		add := false

		if option.tags != nil {
			for _, tag := range taskTags {
				if _, ok := tagMap[tag]; ok {
					add = true
					break
				}
			}
		} else {
			add = true
		}

		if ts := unwrapTask[TaskSet](task); ts != nil {
			if add {
				ts.FilterTasks(func(tasks []Task) []Task {
					return filterTasks(tasks, excludeTags(option.excludeTags...))
				})
			} else {
				ts.FilterTasks(func(tasks []Task) []Task {
					return filterTasks(tasks, opts...)
				})
			}
			add = ts.Len() > 0
		}

		if add {
			filtered = append(filtered, task)
		}
	}

	return filtered
}

func unwrapTask[T any](task Task) T {
	for task != nil {
		if t, ok := task.(T); ok {
			return t
		} else if t, ok := task.(interface{ Unwrap() Task }); ok {
			task = t.Unwrap()
		} else {
			task = nil
		}
	}
	var ret T
	return ret
}
