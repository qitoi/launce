from locust import User, SequentialTaskSet, task, constant


class MyUser(User):
    wait_time = constant(1)

    @task
    class _(SequentialTaskSet):
        @task
        def seq1(self):
            print("task seq1")
            self.schedule_task(self.extra1, True)
            self.schedule_task(self.extra2, True)
            self.schedule_task(self.extra3, False)

        @task
        def seq2(self):
            print("task seq2")

        def extra1(self):
            print("task extra1")

        def extra2(self):
            print("task extra2")

        def extra3(self):
            print("task extra3")
