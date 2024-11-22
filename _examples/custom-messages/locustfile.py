from locust import User, events
from locust.runners import WorkerRunner


def on_pong(environment, msg, **kwargs):
    print(f"receive pong: {msg.data["message"]}")


@events.init.add_listener
def _(environment, **_kwargs):
    if not isinstance(environment.runner, WorkerRunner):
        environment.runner.register_message("pong", on_pong)


@events.test_start.add_listener
def on_test_start(environment, **_kwargs):
    if not isinstance(environment.runner, WorkerRunner):
        for worker in environment.runner.clients.all:
            data = {
                "message": f"ping worker {worker.id}",
            }
            environment.runner.send_message("ping", data, worker.id)


class MyUser(User):
    ...
