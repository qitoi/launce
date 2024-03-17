from locust import User, task, events


@events.init_command_line_parser.add_listener
def _(parser):
    parser.add_argument("--my-arg-int", type=int, env_var="ARG_INT", default=100, help="int argument")
    parser.add_argument("--my-arg-str", type=str, default="hoge", help="str argument")
    parser.add_argument("--my-arg-choice", type=str, choices=["foo", "bar", "baz"], default="foo", help="choice argument")

class MyUser(User):
    @task
    def dummy(self):
        ...
