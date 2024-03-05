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
	"reflect"
	"sync"
	"testing"

	"github.com/qitoi/launce"
)

var (
	_ launce.BaseUser = (*user)(nil)
)

func TestLoadGenerator_OnTestStart(t *testing.T) {
	r := launce.NewLoadGenerator()

	result := false

	r.OnTestStart(func(ctx context.Context) error {
		result = true
		return nil
	})

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	if !result {
		t.Fatal("OnTestStart not called")
	}
}

func TestLoadGenerator_OnTestStart_Error(t *testing.T) {
	r := launce.NewLoadGenerator()

	expected := errors.New("error")

	r.OnTestStart(func(ctx context.Context) error {
		return expected
	})

	if err := r.Start(); !errors.Is(err, expected) {
		t.Fatalf("unexpected result got:%v want:%v", err, expected)
	}
}

func TestLoadGenerator_OnTestStop(t *testing.T) {
	r := launce.NewLoadGenerator()

	result := false

	r.OnTestStop(func(ctx context.Context) {
		result = true
	})

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	r.Stop()

	if !result {
		t.Fatal("OnTestStop not called")
	}
}

func TestLoadGenerator_Spawn(t *testing.T) {
	r := launce.NewLoadGenerator()

	uc := newUserController()

	r.RegisterUser(nil, "TestUser", uc.NewUser)

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	if err := r.Spawn("TestUser", 1); err != nil {
		t.Fatal(err)
	}

	uc.WaitStart(1)

	if n := uc.Started(); n != 1 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 1)
	}
	if n := uc.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 0)
	}

	expectedUsers := map[string]int64{
		"TestUser": 1,
	}
	if users := r.Users(); !reflect.DeepEqual(users, expectedUsers) {
		t.Fatalf("unexpected users. got:%v want:%v", users, expectedUsers)
	}

	if err := r.Spawn("TestUser", 3); err != nil {
		t.Fatal(err)
	}

	uc.WaitStart(3)

	if n := uc.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 3)
	}
	if n := uc.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 0)
	}

	expectedUsers = map[string]int64{
		"TestUser": 3,
	}
	if users := r.Users(); !reflect.DeepEqual(users, expectedUsers) {
		t.Fatalf("unexpected users. got:%v want:%v", users, expectedUsers)
	}

	r.Stop()

	uc.WaitStop(3)

	if n := uc.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 3)
	}
	if n := uc.Stopped(); n != 3 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 3)
	}
}

func TestLoadGenerator_Spawn_MultiUser(t *testing.T) {
	r := launce.NewLoadGenerator()

	uc1 := newUserController()
	uc2 := newUserController()

	r.RegisterUser(nil, "TestUser1", uc1.NewUser)
	r.RegisterUser(nil, "TestUser2", uc2.NewUser)

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	if err := r.Spawn("TestUser1", 2); err != nil {
		t.Fatal(err)
	}
	if err := r.Spawn("TestUser2", 3); err != nil {
		t.Fatal(err)
	}

	uc1.WaitStart(2)
	uc2.WaitStart(3)

	if n := uc1.Started(); n != 2 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 2)
	}
	if n := uc1.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 0)
	}
	if n := uc2.Started(); n != 3 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 3)
	}
	if n := uc2.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 0)
	}

	expectedUsers := map[string]int64{
		"TestUser1": 2,
		"TestUser2": 3,
	}
	if users := r.Users(); !reflect.DeepEqual(users, expectedUsers) {
		t.Fatalf("unexpected users. got:%v want:%v", users, expectedUsers)
	}

	if err := r.Spawn("TestUser2", 5); err != nil {
		t.Fatal(err)
	}

	uc2.WaitStart(5)

	if n := uc1.Started(); n != 2 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 2)
	}
	if n := uc1.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 0)
	}
	if n := uc2.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}
	if n := uc2.Stopped(); n != 0 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 0)
	}

	expectedUsers = map[string]int64{
		"TestUser1": 2,
		"TestUser2": 5,
	}
	if users := r.Users(); !reflect.DeepEqual(users, expectedUsers) {
		t.Fatalf("unexpected users. got:%v want:%v", users, expectedUsers)
	}

	r.Stop()

	uc1.WaitStop(2)
	uc2.WaitStop(5)

	if n := uc1.Started(); n != 2 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 2)
	}
	if n := uc1.Stopped(); n != 2 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 2)
	}
	if n := uc2.Started(); n != 5 {
		t.Fatalf("unexpected started user. got:%v want:%v", n, 5)
	}
	if n := uc2.Stopped(); n != 5 {
		t.Fatalf("unexpected stopped user. got:%v want:%v", n, 5)
	}
}

type user struct {
	launce.BaseUserImpl
	ProcessFunc func(ctx context.Context) error
	OnStartFunc func(ctx context.Context) error
	OnStopFunc  func(ctx context.Context) error
}

func (u *user) Process(ctx context.Context) error {
	if u.ProcessFunc != nil {
		return u.ProcessFunc(ctx)
	}
	return nil
}

func (u *user) OnStart(ctx context.Context) error {
	if u.OnStartFunc != nil {
		return u.OnStartFunc(ctx)
	}
	return nil
}

func (u *user) OnStop(ctx context.Context) error {
	if u.OnStopFunc != nil {
		return u.OnStopFunc(ctx)
	}
	return nil
}

func (u *user) WaitTime() launce.WaitTimeFunc {
	return nil
}

type userController struct {
	started   int
	startCond *sync.Cond
	stopped   int
	stopCond  *sync.Cond
}

func newUserController() *userController {
	return &userController{
		startCond: sync.NewCond(&sync.Mutex{}),
		stopCond:  sync.NewCond(&sync.Mutex{}),
	}
}

func (s *userController) NewUser() launce.User {
	return &user{
		ProcessFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		OnStartFunc: func(ctx context.Context) error {
			s.OnStart()
			return nil
		},
		OnStopFunc: func(ctx context.Context) error {
			s.OnStop()
			return nil
		},
	}
}

func (s *userController) OnStart() {
	s.startCond.L.Lock()
	s.started += 1
	s.startCond.Broadcast()
	s.startCond.L.Unlock()
}

func (s *userController) OnStop() {
	s.stopCond.L.Lock()
	s.stopped += 1
	s.stopCond.Broadcast()
	s.stopCond.L.Unlock()
}

func (s *userController) Started() int {
	s.startCond.L.Lock()
	defer s.startCond.L.Unlock()
	return s.started
}

func (s *userController) Stopped() int {
	s.stopCond.L.Lock()
	defer s.stopCond.L.Unlock()
	return s.stopped
}

func (s *userController) WaitStart(n int) {
	s.startCond.L.Lock()
	for s.started < n {
		s.startCond.Wait()
	}
	s.startCond.L.Unlock()
}

func (s *userController) WaitStop(n int) {
	s.stopCond.L.Lock()
	for s.stopped < n {
		s.stopCond.Wait()
	}
	s.stopCond.L.Unlock()
}
