from locust import User, SequentialTaskSet, tag, task, constant_pacing

import time
import random
import gevent


class MyUser(User):
    wait_time = constant_pacing(0.2)

    @task
    class MyTaskSet(SequentialTaskSet):
        @tag("tag1")
        @task
        def task1(self):
            self.user.environment.events.request.fire(
                request_type="GET",
                name="/task1",
                response_time=None,
                response_length=0,
            )

        @tag("tag2")
        @task
        def task2(self):
            self.user.environment.events.request.fire(
                request_type="GET",
                name="/task2",
                response_time=None,
                response_length=0,
            )

        @tag("tag3")
        @task
        def task3(self):
            self.user.environment.events.request.fire(
                request_type="GET",
                name="/task3",
                response_time=None,
                response_length=0,
            )

        @tag("tag4")
        @task
        def task4(self):
            self.user.environment.events.request.fire(
                request_type="GET",
                name="/task4",
                response_time=None,
                response_length=0,
            )
