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
	"testing"
	"time"

	"github.com/qitoi/launce"
)

func TestWithGracefulStop_DelaysHardCancel(t *testing.T) {
	soft, softCancel := context.WithCancel(context.Background())
	hard, cleanup := launce.WithGracefulStop(soft, 50*time.Millisecond)
	defer cleanup()

	softCancel()

	// ソフト信号は即座に立つ
	if !launce.Stopping(hard) {
		t.Fatal("Stopping should be true immediately after soft is canceled")
	}
	// しかしハードキャンセルはまだ発生していないはず
	if hard.Err() != nil {
		t.Fatal("hard context should not be canceled immediately")
	}

	select {
	case <-hard.Done():
	case <-time.After(1 * time.Second):
		t.Fatal("hard context was not canceled after the grace period")
	}
	if hard.Err() == nil {
		t.Fatal("hard context should be canceled after the grace period")
	}
}

func TestWithGracefulStop_ImmediateWhenNoTimeout(t *testing.T) {
	soft, softCancel := context.WithCancel(context.Background())
	hard, cleanup := launce.WithGracefulStop(soft, 0)
	defer cleanup()

	softCancel()

	select {
	case <-hard.Done():
	case <-time.After(1 * time.Second):
		t.Fatal("hard context should be canceled immediately when stopTimeout is 0")
	}
}

func TestStopping_FalseWithoutGracefulStop(t *testing.T) {
	if launce.Stopping(context.Background()) {
		t.Fatal("Stopping should be false for a plain context")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if launce.Stopping(ctx) {
		t.Fatal("Stopping should be false for a canceled context that wasn't derived via WithGracefulStop")
	}
}
