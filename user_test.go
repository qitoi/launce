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
	"time"

	"github.com/qitoi/launce"
)

var (
	_ launce.BaseUser = (*testUser)(nil)
)

type testUser struct {
	launce.BaseUserImpl
	Start        func(ctx context.Context) error
	Stop         func(ctx context.Context) error
	Func         func(ctx context.Context) error
	WaitDuration time.Duration
}

func (t *testUser) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(t.WaitDuration)
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
	err := launce.ProcessUser(context.Background(), &u, 0)
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
	err := launce.ProcessUser(context.Background(), &u, 0)
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
			if count == 1 || count == 2 {
				return errUser
			} else if count == 3 {
				return launce.StopUser
			}
			return nil
		},
	}
	err := launce.ProcessUser(context.Background(), &u, 0)
	if err != nil {
		t.Fatalf("unexpected result error. got:%v want:%v", err, errUser)
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

// TestProcessUser_GracefulStop は、stopTimeout が設定されている場合、ソフトな
// 停止合図 (ctx のキャンセル) が来ても実行中の Process 呼び出しは中断されず、
// 完了してから次の Process を呼ばずに終了することを確認する。
func TestProcessUser_GracefulStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	stop := 0
	processCount := 0
	var hardCanceledDuringProcess bool

	inProcess := make(chan struct{})
	release := make(chan struct{})

	u := testUser{
		Stop: func(ctx context.Context) error {
			stop += 1
			return nil
		},
		Func: func(ctx context.Context) error {
			processCount += 1
			if processCount == 1 {
				close(inProcess)
				<-release
				hardCanceledDuringProcess = ctx.Err() != nil
			}
			return nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- launce.ProcessUser(ctx, &u, 1*time.Second)
	}()

	<-inProcess
	cancel()
	time.Sleep(20 * time.Millisecond)
	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected result error. got:%v want:%v", err, nil)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("ProcessUser did not return")
	}

	if hardCanceledDuringProcess {
		t.Fatal("the in-flight Process call should not observe cancellation during the grace period")
	}
	if processCount != 1 {
		t.Fatalf("Process should not be called again after a soft stop. got:%d want:%d", processCount, 1)
	}
	if stop != 1 {
		t.Fatalf("unexpected user stop count error. got:%v want:%v", stop, 1)
	}
}

// TestProcessUser_GracefulStop_HardCancelAfterTimeout は、猶予期間内に
// Process が終わらなければ、stopTimeout 経過後に実際にキャンセルされ、
// それを見た Process が中断できることを確認する。
func TestProcessUser_GracefulStop_HardCancelAfterTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	stop := 0
	var hardCanceled bool

	u := testUser{
		Stop: func(ctx context.Context) error {
			stop += 1
			return nil
		},
		Func: func(ctx context.Context) error {
			<-ctx.Done()
			hardCanceled = true
			return ctx.Err()
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- launce.ProcessUser(ctx, &u, 20*time.Millisecond)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected result error. got:%v want:%v", err, nil)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("graceful stop did not escalate to a hard cancel within the grace period")
	}

	if !hardCanceled {
		t.Fatal("hard context should have been canceled after the grace period")
	}
	if stop != 1 {
		t.Fatalf("unexpected user stop count error. got:%v want:%v", stop, 1)
	}
}

// TestBaseUserImpl_Wait_InterruptedBySoftStop は、猶予期間中であっても
// (実行中のタスクとは違い) 単に待機しているだけの Wait は、次のタスクを
// 開始する前のソフト信号で即座に中断されることを確認する。
func TestBaseUserImpl_Wait_InterruptedBySoftStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	u := &testUser{WaitDuration: time.Hour}
	u.Init(u, nil, nil)

	started := make(chan struct{})
	waitErr := make(chan error, 1)
	u.Func = func(ctx context.Context) error {
		close(started)
		err := u.Wait(ctx)
		waitErr <- err
		return err
	}

	done := make(chan error, 1)
	go func() {
		// stopTimeout を長くして、hard cancel では絶対に起きないようにする
		done <- launce.ProcessUser(ctx, u, time.Hour)
	}()

	<-started
	start := time.Now()
	cancel()

	select {
	case err := <-waitErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error. got:%v want:%v", err, context.Canceled)
		}
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Fatalf("Wait took too long to be interrupted by soft stop. elapsed:%s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait was not interrupted by soft stop")
	}

	<-done
}
