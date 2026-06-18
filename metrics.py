from config import get_config
import psutil


class Metrics:
    def __init__(self, name, current, warning, critical):
        self.name = name
        self.current = current
        self.warning = warning
        self.critical = critical


def get_metrics():

    cpu_percent = psutil.cpu_percent(interval=1)
    ram_percent = psutil.virtual_memory().percent
    disk_percent = psutil.disk_usage("/").percent

    threshold = get_config()

    cpu = Metrics(
        "CPU",
        cpu_percent,
        threshold["CPU"][0]["warning"],
        threshold["CPU"][0]["critical"],
    )

    ram = Metrics(
        "RAM",
        ram_percent,
        threshold["RAM"][0]["warning"],
        threshold["RAM"][0]["critical"],
    )

    disk = Metrics(
        "Disk",
        disk_percent,
        threshold["Disk"][0]["warning"],
        threshold["Disk"][0]["critical"],
    )

    return cpu, ram, disk
