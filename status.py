from metrics import get_metrics


def get_status():
    cpu, ram, disk = get_metrics()

    cpu.print_status()
    ram.print_status()
    disk.print_status()
