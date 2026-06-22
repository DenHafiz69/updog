# Check health and log if needed
from metrics import get_metrics
import logging


def initialize_logger():
    logging.basicConfig(
        filename="updog.log",
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
    )


def get_check():

    initialize_logger()

    cpu, ram, disk = get_metrics()
    metrics = [cpu, ram, disk]

    for metric in metrics:
        if metric.check_status() == 0:
            logging.info(f"{metric.log()}")
        elif metric.check_status() == 1:
            logging.warning(f"{metric.log()}")
        else:
            logging.critical(f"{metric.log()}")
