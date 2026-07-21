from locust import User, task, constant

import datetime
import time
import gevent

# タスク1回あたりの所要時間。--stop-timeout をこれより長く指定すると、
# 停止要求が来てもタスクが最後まで完了するようになる。
TASK_DURATION = 3.0

id_counter = 0


def now():
    return datetime.datetime.now().strftime("%H:%M:%S.%f")[:-3]


class MyUser(User):
    wait_time = constant(1)

    def on_start(self):
        global id_counter
        id_counter += 1
        self.id = id_counter

    @task
    def process(self):
        print(f"[{now()}] id={self.id} task 開始")

        start = time.perf_counter()
        gevent.sleep(TASK_DURATION)
        response_time = (time.perf_counter() - start) * 1000

        print(f"[{now()}] id={self.id} task 完了 (経過 {response_time / 1000:.1f}s)")

        self.environment.events.request.fire(
            request_type="GET",
            name="/task",
            response_time=response_time,
            response_length=0,
        )

        self.wait()
