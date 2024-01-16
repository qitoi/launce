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
	"context"
	"errors"
	"testing"

	"github.com/qitoi/launce"
)

type testUser struct {
	launce.BaseUser
	Start func(ctx context.Context) error
	Stop  func(ctx context.Context) error
	Func  func(ctx context.Context) error
	Value int
}

func (t *testUser) OnStart(ctx context.Context) error {
	if t.Start == nil {
		return nil
	}
	return t.Start(ctx)
}

func (t *testUser) OnStop(ctx context.Context) error {
	if t.Stop == nil {
		return nil
	}
	return t.Stop(ctx)
}

func (t *testUser) Process(ctx context.Context) error {
	return t.Func(ctx)
}

func TestProcessUser_StopUser(t *testing.T) {
	start := 0
	stop := 0
	count := 0
	u := testUser{
		Start: func(ctx context.Context) error {
			start += 1
			return nil
		},
		Stop: func(ctx context.Context) error {
			stop += 1
			return nil
		},
		Func: func(ctx context.Context) error {
			count += 1
			if count == 3 {
				return launce.StopUser
			}
			return nil
		},
	}
	err := launce.ProcessUser(context.Background(), &u)
	if err != nil {
		t.Fatalf("unexpected result error. got:%v want:%v", err, nil)
	}
	if start != 1 {
		t.Fatalf("unexpected user start count error. got:%v want:%v", start, 1)
	}
	if stop != 1 {
		t.Fatalf("unexpected user stop count error. got:%v want:%v", stop, 1)
	}
	if count != 3 {
		t.Fatalf("unexpected user loop count error. got:%v want:%v", count, 3)
	}
}

func TestProcessUser_ContextCancel(t *testing.T) {
	start := 0
	stop := 0
	count := 0
	u := testUser{
		Start: func(ctx context.Context) error {
			start += 1
			return nil
		},
		Stop: func(ctx context.Context) error {
			stop += 1
			return nil
		},
		Func: func(ctx context.Context) error {
			count += 1
			if count == 3 {
				return context.Canceled
			}
			return nil
		},
	}
	err := launce.ProcessUser(context.Background(), &u)
	if err != nil {
		t.Fatalf("unexpected result error. got:%v want:%v", err, nil)
	}
	if start != 1 {
		t.Fatalf("unexpected user start count error. got:%v want:%v", start, 1)
	}
	if stop != 1 {
		t.Fatalf("unexpected user stop count error. got:%v want:%v", stop, 1)
	}
	if count != 3 {
		t.Fatalf("unexpected user loop count error. got:%v want:%v", count, 3)
	}
}

func TestProcessUser_UnexpectedError(t *testing.T) {
	errUser := errors.New("user error")
	start := 0
	stop := 0
	count := 0
	u := testUser{
		Start: func(ctx context.Context) error {
			start += 1
			return nil
		},
		Stop: func(ctx context.Context) error {
			stop += 1
			return nil
		},
		Func: func(ctx context.Context) error {
			count += 1
			if count == 3 {
				return errUser
			}
			return nil
		},
	}
	err := launce.ProcessUser(context.Background(), &u)
	if !errors.Is(err, errUser) {
		t.Fatalf("unexpected result error. got:%v want:%v", err, errUser)
	}
	if start != 1 {
		t.Fatalf("unexpected user start count error. got:%v want:%v", start, 1)
	}
	if stop != 0 {
		t.Fatalf("unexpected user stop count error. got:%v want:%v", stop, 0)
	}
	if count != 3 {
		t.Fatalf("unexpected user loop count error. got:%v want:%v", count, 3)
	}
}
