from config import get_config
import psutil


class Metrics:
    def __init__(self, name, current_percentage, threshold):
        self.name = name
        self.current_percentage = current_percentage
        self.threshold = threshold


def get_metrics():

    cpu_percent = psutil.cpu_percent(interval=1)
    ram_percent = psutil.virtual_memory().percent
    disk_percent = psutil.disk_usage("/").percent

    default_threshold = get_config()

    cpu = Metrics("CPU", cpu_percent, default_threshold["cpu_threshold"])
    ram = Metrics("RAM", ram_percent, default_threshold["ram_threshold"])
    disk = Metrics("Disk", disk_percent, default_threshold["disk_threshold"])

    return cpu, ram, disk
