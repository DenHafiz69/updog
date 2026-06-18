import psutil
from emoji import emojize


def get_status():
    print("This is the status.")

    check_mark = emojize(":check_mark_button:")
    exclamation_mark = emojize(":red_exclamation_mark:")

    cpu_percent = psutil.cpu_percent(interval=1)
    ram_percent = psutil.virtual_memory().percent
    disk_percent = psutil.disk_usage("/").percent

    print(f"{check_mark}, {exclamation_mark}")

    return cpu_percent, ram_percent, disk_percent
