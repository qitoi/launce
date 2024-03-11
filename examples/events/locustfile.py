from locust import User, task

class TestUser(User):
    @task
    def dummy(self):
        ...
