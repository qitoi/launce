from locust import User, TaskSet, SequentialTaskSet, task, events, constant


@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    print("Worker OnTestStart")

@events.test_stopping.add_listener
def on_test_stopping(environment, **kwargs):
    print("Worker OnTestStopping")

@events.test_stop.add_listener
def on_test_stop(environment, **kwargs):
    print("Worker OnTestStop")

@events.quitting.add_listener
def on_quitting(**kwargs):
    print("Worker OnQuitting")

@events.quit.add_listener
def on_quit(**kwargs):
    print("Worker OnQuit")

class MyTaskSet(TaskSet):
    def on_start(self):
        print(f"TaskSet OnStart")

    def on_stop(self):
        print(f"TaskSet OnStop")

    @task(3)
    def task1(self):
        print("TaskSet task1")

    @task(2)
    class SubTaskSet(SequentialTaskSet):
        def on_start(self):
            print(f"SubTaskSet OnStart")

        def on_stop(self):
            print(f"SubTaskSet OnStop")

        @task
        def task1(self):
            print("SubTaskSet task1")

        @task
        def task2(self):
            print("SubTaskSet task2")

        @task
        def quit(self):
            print("SubTaskSet quit")
            self.interrupt(False)

    @task(1)
    def quit(self):
        print("TaskSet quit")
        self.interrupt(False)


class MyUser(User):
    wait_time = constant(1)
    tasks = [MyTaskSet]
    id = 0

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        MyUser.id += 1
        self.id = MyUser.id

    def on_start(self):
        print(f"User #{self.id} OnStart")

    def on_stop(self):
        print(f"User #{self.id} OnStop")
