from locust import User, task

class MyUser(User):
    @task
    def dummy(self):
        ...
