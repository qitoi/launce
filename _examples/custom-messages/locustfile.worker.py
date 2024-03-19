from locust import User, task, events, constant


def handle_ping(environment, msg, **kwargs):
    print(f"receive ping: {msg.data["message"]}")
    environment.runner.send_message("pong", {"message": f"pong worker {environment.runner.client_id}"})


@events.init.add_listener
def on_locust_init(environment, **kwargs):
    environment.runner.register_message("ping", handle_ping)


class MyUser(User):
    wait_time = constant(1)

    @task
    def _(self):
        pass
