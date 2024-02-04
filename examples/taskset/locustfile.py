import time
from locust import User, task

class TasksetUser(User):
    @task
    def dummy(self):
        ...
