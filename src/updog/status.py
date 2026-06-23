from updog.metrics import get_metrics


def flag_check(args):
    cpu, ram, disk = get_metrics()

    if args[2].lower() == "cpu":
        cpu.print_status()
    elif args[2].lower() == "ram":
        ram.print_status()
    elif args[2].lower() == "disk":
        disk.print_status()
    else:
        raise ValueError("Only accepts value of 'cpu', 'ram', or 'disk'.")


def get_status(args):

    if len(args) == 3:
        flag_check(args)
    else:
        cpu, ram, disk = get_metrics()
        cpu.print_status()
        ram.print_status()
        disk.print_status()
