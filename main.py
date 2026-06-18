from config import print_config
from check import get_check
from alert import get_alert
from logs import get_logs
from status import get_status
import sys


def main():
    if len(sys.argv) == 1:
        print("Not enough argument.")
    elif sys.argv[1] == "status":
        get_status()
    elif sys.argv[1] == "logs":
        get_logs()
    elif sys.argv[1] == "alert":
        get_alert()
    elif sys.argv[1] == "check":
        get_check()
    elif sys.argv[1] == "config":
        print_config()


if __name__ == "__main__":
    main()
