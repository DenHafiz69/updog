from metrics import get_metrics, Metrics
from emoji import emojize


def check_status(metric: Metrics):
    # Emoji for showing status
    check_mark = emojize(":check_mark_button:")
    exclamation_mark = emojize(":red_exclamation_mark:")

    if metric.current_percentage <= metric.threshold:
        print(f"[{check_mark}] {metric.name}: {metric.current_percentage}")
    else:
        print(f"[{exclamation_mark}] {metric.name}: {metric.current_percentage}")
    pass


def get_status():
    print("This is the status.")

    cpu, ram, disk = get_metrics()

    check_status(cpu)
