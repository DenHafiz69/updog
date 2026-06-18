from metrics import get_metrics, Metrics
from emoji import emojize


def check_status(metric: Metrics):
    # Emoji for showing status
    check_mark = emojize(":check_mark_button:")
    exclamation_mark = emojize(":red_exclamation_mark:")
    warning = emojize(":warning:")

    if metric.current <= metric.warning:
        print(f"[{check_mark}] {metric.name}: {metric.current}% (Healthy)")
        return "healthy"
    elif metric.current > metric.warning and metric.current <= metric.critical:
        print(
            f"[{warning}] {metric.name}: {metric.current}% (WARNING: Exceeds {metric.warning}% threshold)"
        )
        return "warning"
    else:
        print(
            f"[{exclamation_mark}] {metric.name}: {metric.current}% (CRITICAL: Exceeds {metric.critical}% threshold.)"
        )
        return "critical"


def get_status():
    cpu, ram, disk = get_metrics()

    check_status(cpu)
    check_status(ram)
    check_status(disk)
