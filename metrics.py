from config import get_config
from emoji import emojize
import psutil


class Metrics:
    def __init__(self, name, current, warning, critical):
        self.name = name
        self.current = current
        self.warning = warning
        self.critical = critical

    def check_status(self):
        if self.current <= self.warning:
            return 0  # 0 is healthy
        elif self.current <= self.critical:
            return 1  # 1 is warning
        else:
            return 2  # 2 is critical

    def log(self):
        if self.check_status() == 0:
            return f"{self.name}: {self.current}% is within thresholds"
        elif self.check_status() == 1:
            return f"{self.name}: {self.current}% exceeds {self.warning}% warning threshold"
        else:
            return f"{self.name}: {self.current}% exceeds {self.critical}% critical threshold"

    def print_status(self):

        check_mark = emojize(":check_mark_button:")
        exclamation_mark = emojize(":red_exclamation_mark:")
        warning = emojize(":warning:")

        status = self.check_status()

        if status == 0:
            print(f"[{check_mark}] {self.name}: {self.current}% (Healthy)")
            return "healthy"
        elif status == 1:
            print(
                f"[{warning}] {self.name}: {self.current}% (WARNING: Exceeds {self.warning}% threshold)"
            )
            return "warning"
        else:
            print(
                f"[{exclamation_mark}] {self.name}: {self.current}% (CRITICAL: Exceeds {self.critical}% threshold.)"
            )
            return "critical"


def get_metrics():

    cpu_percent = psutil.cpu_percent(interval=1)
    ram_percent = psutil.virtual_memory().percent
    disk_percent = psutil.disk_usage("/").percent

    threshold = get_config()

    cpu = Metrics(
        "CPU",
        cpu_percent,
        threshold["CPU"]["warning"],
        threshold["CPU"]["critical"],
    )

    ram = Metrics(
        "RAM",
        ram_percent,
        threshold["RAM"]["warning"],
        threshold["RAM"]["critical"],
    )

    disk = Metrics(
        "Disk",
        disk_percent,
        threshold["Disk"]["warning"],
        threshold["Disk"]["critical"],
    )

    return cpu, ram, disk
