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

// stop-timeout は、master の --stop-timeout オプションによる
// グレースフルストップ (graceful stop) の挙動を確認するサンプルです。
//
// MyUser の Process は 3 秒かかるタスクを実行します。-t 6s は 2 回目のタスク
// (4〜7 秒) の途中で run-time が終わるタイミングです。ログの「経過」が 3s なら
// stop-timeout により graceful stop され完走したこと、3s 未満なら猶予なく
// 中断されたことを示します。
//
//	locust -f locustfile.py --master --headless -u 3 -r 3 -t 6s --expect-workers 1 --stop-timeout 5
//	go run .
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/qitoi/launce"
)

func main() {
	transport := launce.NewZmqTransport("localhost", 5557)
	worker, err := launce.NewWorker(transport)
	if err != nil {
		log.Fatal(err)
	}

	worker.RegisterUser("MyUser", func() launce.User {
		return &User{}
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
