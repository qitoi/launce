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
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/qitoi/launce"
	"github.com/qitoi/launce/stats"
)

var (
	_ launce.BaseUserRequirement = (*user)(nil)
)

func TestLoadRunner_OnTestStart(t *testing.T) {
	r := launce.NewLoadRunner()

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

func TestLoadRunner_OnTestStart_Error(t *testing.T) {
	r := launce.NewLoadRunner()

	expected := errors.New("error")

	r.OnTestStart(func(ctx context.Context) error {
		return expected
	})

	if err := r.Start(); !errors.Is(err, expected) {
		t.Fatalf("unexpected result got:%v want:%v", err, expected)
	}
}

func TestLoadRunner_OnTestStop(t *testing.T) {
	r := launce.NewLoadRunner()

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

func TestLoadRunner_Spawn(t *testing.T) {
	r := launce.NewLoadRunner()

	uc := newUserController()

	r.RegisterUser("TestUser", uc.NewUser)

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

func TestLoadRunner_Spawn_MultiUser(t *testing.T) {
	r := launce.NewLoadRunner()

	uc1 := newUserController()
	uc2 := newUserController()

	r.RegisterUser("TestUser1", uc1.NewUser)
	r.RegisterUser("TestUser2", uc2.NewUser)

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

func TestLoadRunner_Report(t *testing.T) {
	r := launce.NewLoadRunner()

	r.Report("GET", "/foo", launce.WithResponseTime(100*time.Microsecond), launce.WithResponseLength(1024))
	r.Report("GET", "/foo", launce.WithResponseTime(200*time.Microsecond), launce.WithResponseLength(10), launce.WithError(errors.New("error")))
	r.Report("GET", "/bar", launce.WithResponseTime(100*time.Microsecond), launce.WithResponseLength(1024))

	entries, total, errs := r.FlushStats()

	if n := entries[stats.EntryKey{Method: "GET", Name: "/foo"}].NumRequests; n != 2 {
		t.Fatalf("foo.num_requests got:%v want:%v", n, 2)
	}
	if n := entries[stats.EntryKey{Method: "GET", Name: "/bar"}].NumRequests; n != 1 {
		t.Fatalf("bar.num_requests got:%v want:%v", n, 1)
	}
	if n := total.NumRequests; n != 3 {
		t.Fatalf("foo.num_requests got:%v want:%v", n, 3)
	}
	if n := errs[stats.ErrorKey{Method: "GET", Name: "/foo", Error: "error"}]; n != 1 {
		t.Fatalf("foo.errors got:%v want:%v", n, 3)
	}
}

func TestLoadRunner_Host(t *testing.T) {
	host := "https://example.com/"
	r := launce.NewLoadRunner()
	r.SetHost(host)

	if h := r.Host(); h != host {
		t.Fatalf("host got:%v want:%v", h, host)
	}

	ch := make(chan string)

	r.RegisterUser("TestUser", func() launce.User {
		u := &user{}
		u.ProcessFunc = func(ctx context.Context) error {
			ch <- u.Runner().Host()
			close(ch)
			return launce.StopUser
		}
		return u
	})

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	if err := r.Spawn("TestUser", 1); err != nil {
		t.Fatal(err)
	}

	if h := <-ch; h != host {
		t.Fatalf("host got:%v want:%v", h, host)
	}
}

func TestLoadRunner_ParsedOptions(t *testing.T) {
	masterHost := "localhost"
	tags := []string{"tag1", "tag2"}

	options := launce.ParsedOptions{
		Tags:       &tags,
		MasterHost: masterHost,
	}

	r := launce.NewLoadRunner()
	r.SetParsedOptions(&options)

	p := r.ParsedOptions()
	if p.MasterHost != masterHost {
		t.Fatalf("ParsedOptions.MasterHost got:%v want:%v", p.MasterHost, masterHost)
	}
	if p.Tags == nil {
		t.Fatalf("ParsedOptions.Tags got:%v want:%v", p.Tags, tags)
	} else if slices.Compare(*p.Tags, tags) != 0 {
		t.Fatalf("ParsedOptions.Tags got:%v want:%v", *p.Tags, tags)
	}

	ch := make(chan *launce.ParsedOptions)

	r.RegisterUser("TestUser", func() launce.User {
		u := &user{}
		u.ProcessFunc = func(ctx context.Context) error {
			ch <- u.Runner().ParsedOptions()
			close(ch)
			return launce.StopUser
		}
		return u
	})

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	if err := r.Spawn("TestUser", 1); err != nil {
		t.Fatal(err)
	}

	p = <-ch

	if p.MasterHost != masterHost {
		t.Fatalf("ParsedOptions.MasterHost got:%v want:%v", p.MasterHost, masterHost)
	}
	if p.Tags == nil {
		t.Fatalf("ParsedOptions.Tags got:%v want:%v", p.Tags, tags)
	} else if slices.Compare(*p.Tags, tags) != 0 {
		t.Fatalf("ParsedOptions.Tags got:%v want:%v", *p.Tags, tags)
	}
}

func TestLoadRunner_Message(t *testing.T) {
	r := launce.NewLoadRunner()

	type Payload struct {
		Message string
	}

	ch := make(chan Payload, 1)

	r.RegisterMessage("custom-message", func(msg launce.ReceivedMessage) {
		var payload Payload
		_ = msg.DecodePayload(&payload)
		ch <- payload
		close(ch)
	})

	r.SendMessageFunc = func(typ string, data any) error {
		msg := launce.Message{
			Type:   typ,
			Data:   data,
			NodeID: "",
		}
		b, _ := launce.EncodeMessage(msg)
		decMsg, _ := launce.DecodeMessage(b)
		r.HandleMessage(decMsg)
		return nil
	}

	_ = r.SendMessage("custom-message", Payload{Message: "foo"})

	if payload := <-ch; payload.Message != "foo" {
		t.Fatalf("received message got:%v want:%v", payload.Message, "foo")
	}
}

type user struct {
	launce.BaseUser
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
