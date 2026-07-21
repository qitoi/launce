from locust import User, task, constant

import logging
import time
import random
import gevent

logger = logging.getLogger(__name__)


class MyUser(User):
    wait_time = constant(1)

    @task
    def process(self):
        t = time.perf_counter()
        # do something
        gevent.sleep(random.random() * 1)
        response_time = (time.perf_counter() - t) * 1000
        content_length = random.randrange(1024 * 1024)

        # Locust のワーカーはログ出力を自動的にキャプチャしてマスターに転送するため launce のように明示的な設定は不要
        logger.info(f"request finished name=/foo response_time={response_time} content_length={content_length}")

        self.environment.events.request.fire(
            request_type="GET",
            name="/foo",
            response_time=response_time,
            response_length=content_length,
        )

        self.wait()
