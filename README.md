
# launce

Launce is a Locust worker library written in Go.
The aim of this library is to write load test scenarios as simply as locustfile.py and to run load testing with better performance.


## Supported Features

| Locust Feature      | Supported | Example                                                    |
|---------------------|-----------|------------------------------------------------------------|
| WorkerRunner        | Yes       | [_examples/simple-user](./_examples/simple-user)           |
| MasterRunner        | No        |                                                            |
| LocalRunner         | No        |                                                            |
| User                | Yes       | [_examples/simple-user](./_examples/simple-user)           |
| HttpUser            | No        |                                                            |
| TaskSet             | Yes       | [_examples/taskset](./_examples/taskset)                   |
| SequentialTaskSet   | Yes       | [_examples/taskset](./_examples/taskset)                   |
| tag decorator       | Yes       | [_examples/tagged-task](./_examples/tagged-task)           |
| wait_time functions | Yes       |                                                            |
| custom messages     | Yes       | [_examples/custom-messages](./_examples/custom-messages)   |
| custom arguments    | Yes       | [_examples/custom-arguments](./_examples/custom-arguments) |


## Install

```sh
go get github.com/qitoi/launce
```


## Usage

You need to write two scripts, a master script and a worker script.


### Master Script

Master script must define user class, but is not meant to implement user behavior.

```python
# locustfile.py
from locust import User, task

class MyUser(User):
    @task
    def dummy(self):
        ...
```

### Worker Script

Worker script is where you implement the actual load test scenario.

```go
// main.go
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/qitoi/launce"
)

var (
	_ launce.BaseUser = (*MyUser)(nil)
)

// MyUser implements custom user behavior for load testing
type MyUser struct {
	// Inherits default methods from BaseUserImpl
	launce.BaseUserImpl
	host   string
	client *http.Client
}

// WaitTime defines how long the user waits by Wait method
func (u *MyUser) WaitTime() launce.WaitTimeFunc {
	return launce.Constant(1 * time.Second)
}

// OnStart is called when the user starts
func (u *MyUser) OnStart(ctx context.Context) error {
	u.host = u.Runner().Host()
	u.client = &http.Client{}
	return nil
}

// Process defines user action
func (u *MyUser) Process(ctx context.Context) error {
	if err := u.request(http.MethodGet, "/hello"); err != nil {
		return err
	}

	if err := u.Wait(ctx); err != nil {
		return err
	}

	return nil
}

func (u *MyUser) request(method, path string) error {
	req, err := http.NewRequest(method, u.host+path, nil)
	if err != nil {
		return err
	}

	t := time.Now()

	res, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// count response size
	size, err := io.Copy(io.Discard, res.Body)
	if err != nil {
		return err
	}

	responseTime := time.Since(t)

	var resErr error
	if 400 <= res.StatusCode && res.StatusCode < 600 {
		resErr = errors.New("unexpected response status code")
	}

	// report stats of a request to master
	u.Report(method, path, responseTime, size, resErr)

	return nil
}

func main() {
	// initializes a ZeroMQ transport for communication with the Locust master
	transport := launce.NewZmqTransport("localhost", 5557)
	// create worker
	worker, err := launce.NewWorker(transport)
	if err != nil {
		log.Fatal(err)
	}

	// register user
	worker.RegisterUser("MyUser", func() launce.User {
		return &MyUser{}
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		worker.Quit()
	}()

	// start worker
	if err := worker.Join(); err != nil {
		log.Fatal(err)
	}
}
```

Run master and worker.

```sh
# run Locust master
locust --master -f locustfile.py

# run Locust worker by launce (in another terminal)
go run .
```


## License

Apache License 2.0

```
Copyright 2024 qitoi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
