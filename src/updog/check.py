# Check health and log if needed
from updog.metrics import get_metrics
import logging
from time import sleep


def initialize_logger():
    logging.basicConfig(
        filename="updog.log",
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
    )


def get_check(args):

    initialize_logger()

    loop_counts = 1

    if len(args) == 3:
        try:
            loop_counts = int(args[2])
        except ValueError:
            raise ValueError("Argument must be an integer")

    cpu, ram, disk = get_metrics()
    metrics = [cpu, ram, disk]

    for _ in range(loop_counts):
        for metric in metrics:
            if metric.check_status() == 0:
                logging.info(f"{metric.log()}")
            elif metric.check_status() == 1:
                logging.warning(f"{metric.log()}")
            else:
                logging.critical(f"{metric.log()}")

        sleep(1)  # Sleep for 1 second
