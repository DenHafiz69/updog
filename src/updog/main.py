from config import print_config, set_config, generate_default
from check import get_check
from alert import get_alert
from logs import get_logs
from status import get_status
import sys


def main():
    if len(sys.argv) == 1:
        print("Not enough argument.")
    elif sys.argv[1] == "status":
        get_status(sys.argv)
    elif sys.argv[1] == "logs":
        get_logs(sys.argv)
    elif sys.argv[1] == "alert":
        get_alert()
    elif sys.argv[1] == "check":
        get_check(sys.argv)
    elif sys.argv[1] == "config":
        if "--set-critical" in sys.argv:
            index = sys.argv.index("--set-critical")
            metric = sys.argv[index + 1]
            percent = sys.argv[index + 2]
            set_config("critical", metric, int(percent))
        elif "--set-warning" in sys.argv:
            index = sys.argv.index("--set-warning")
            metric = sys.argv[index + 1]
            percent = sys.argv[index + 2]
            set_config("warning", metric, int(percent))
        elif "--default" in sys.argv:
            generate_default()
        else:
            print_config()


if __name__ == "__main__":
    main()
