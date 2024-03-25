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
	"fmt"
	"log"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/qitoi/launce"
)

func main() {
	transport := launce.NewZmqTransport("localhost", 5557)
	worker, err := launce.NewWorker(transport)
	if err != nil {
		log.Fatal(err)
	}

	var id atomic.Int64
	worker.RegisterUser("MyUser", func() launce.User {
		return &User{
			ID: id.Add(1),
		}
	})

	// launce only
	worker.OnConnect(func() {
		fmt.Println("Worker OnConnect")
	})

	worker.OnTestStart(func(ctx context.Context) error {
		fmt.Println("Worker OnTestStart")
		return nil
	})

	worker.OnTestStopping(func(ctx context.Context) {
		fmt.Println("Worker OnTestStopping")
	})

	worker.OnTestStop(func(ctx context.Context) {
		fmt.Println("Worker OnTestStop")
	})

	worker.OnQuitting(func() {
		fmt.Println("Worker OnQuitting")
	})

	worker.OnQuit(func() {
		fmt.Println("Worker OnQuit")
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		worker.Quit()
	}()

	if err := worker.Join(); err != nil {
		log.Fatal(err)
	}
}
