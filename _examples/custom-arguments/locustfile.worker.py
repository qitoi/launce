from locust import User, task, events, constant


@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    args = {
        "Int": environment.parsed_options.my_arg_int,
        "Str": environment.parsed_options.my_arg_str,
        "Choice": environment.parsed_options.my_arg_choice,
    }
    print(f"Custom Arguments: {args}")


class MyUser(User):
    wait_time = constant(1)

    def on_start(self):
        self.str = self.environment.parsed_options.my_arg_str

    @task
    def task(self):
        print(f"user task {self.str}")
