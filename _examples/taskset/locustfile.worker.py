from locust import User, SequentialTaskSet, tag, task, constant_pacing

import time
import random
import gevent


class MyUser(User):
    wait_time = constant_pacing(0.2)

    @task
    def task1(self):
        self.environment.events.request.fire(
            request_type="GET",
            name="/task1",
            response_time=None,
            response_length=0,
        )

    @task
    def task2(self):
        self.environment.events.request.fire(
            request_type="GET",
            name="/task2",
            response_time=None,
            response_length=0,
        )

    @task
    class MyTaskSet(SequentialTaskSet):
        @task
        def task1(self):
            self.user.environment.events.request.fire(
                request_type="GET",
                name="/seq/1",
                response_time=None,
                response_length=0,
            )

        @task
        def task2(self):
            self.user.environment.events.request.fire(
                request_type="GET",
                name="/seq/2",
                response_time=None,
                response_length=0,
            )

        @task
        def task3(self):
            self.user.environment.events.request.fire(
                request_type="GET",
                name="/seq/3",
                response_time=None,
                response_length=0,
            )
            self.interrupt(False)
