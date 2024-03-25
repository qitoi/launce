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
	"syscall"

	"github.com/qitoi/launce"
)

type CustomArguments struct {
	Int    int64  `msgpack:"my_arg_int"`
	Str    string `msgpack:"my_arg_str"`
	Choice string `msgpack:"my_arg_choice"`
}

func main() {
	transport := launce.NewZmqTransport("localhost", 5557)
	worker, err := launce.NewWorker(transport)
	if err != nil {
		log.Fatal(err)
	}

	worker.RegisterUser("MyUser", func() launce.User {
		return &User{}
	})

	worker.OnTestStart(func(ctx context.Context) error {
		var args CustomArguments
		if err := worker.Options(&args); err != nil {
			return err
		}
		fmt.Printf("Custom Arguments: %+v\n", args)
		return nil
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
