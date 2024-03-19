from locust import User, task, constant

import time
import random
import gevent


class MyUser(User):
    wait_time = constant(1)

    @task
    def process(self):
        t = time.perf_counter()
        # do something
        gevent.sleep(random.random() * 1)
        response_time = (time.perf_counter() - t) * 1000
        content_length = random.randrange(1024 * 1024)

        self.environment.events.request.fire(
            request_type="GET",
            name="/foo",
            response_time=response_time,
            response_length=content_length,
        )

        self.wait()

        t = time.perf_counter()
        # do something
        gevent.sleep(random.random() * 1)
        response_time = (time.perf_counter() - t) * 1000
        content_length = random.randrange(1024 * 1024)

        self.environment.events.request.fire(
            request_type="GET",
            name="/bar",
            response_time=response_time,
            response_length=content_length,
            exception=Exception("unexpected response status code"),
        )

        self.wait()
