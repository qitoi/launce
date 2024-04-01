from locust import User, task, constant


class MyUser(User):
    wait_time = constant(1)

    @task
    def task1(self):
        raise Exception("unknown error")
