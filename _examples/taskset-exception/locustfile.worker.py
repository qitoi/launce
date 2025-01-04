from locust import User, SequentialTaskSet, events, task, constant
from locust.runners import MasterRunner

@events.init.add_listener
def on_locust_init(environment, **kwargs):
    environment.catch_exceptions = False


class MyUser(User):
    wait_time = constant(1)

    def on_start(self):
        print("User OnStart")

    def on_stop(self):
        print("User OnStop")

    @task
    def task1(self):
        print("User Task 1")

    @task
    class MyTaskSet(SequentialTaskSet):
        def on_start(self):
            print("TaskSet OnStart")

        def on_stop(self):
            print("TaskSet OnStop")

        @task
        def task1(self):
            print("TaskSet Task 1")

        @task
        def task2(self):
            print("TaskSet Task 2")
            raise Exception("error")

        @task
        def task3(self):
            print("TaskSet Task 3")
