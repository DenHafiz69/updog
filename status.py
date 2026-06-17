import psutil


def get_status():
    print("This is the status.")

    cpu_percent = psutil.cpu_percent(interval=1)
    ram_percent = psutil.virtual_memory().percent
    disk_percent = psutil.disk_usage("/").percent

    return cpu_percent, ram_percent, disk_percent
