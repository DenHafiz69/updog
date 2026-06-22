from pathlib import Path
import json
import os


# def set_config(flag: str, metric: str, percent: float):
def set_config(percent):

    # User call with flag --set-warning or --set-critical
    # Then followed by the percentage
    # warning must be less than critical
    config_data = get_config()
    config_file_path = get_file_path()

    # Edit the config here
    config_data["CPU"]["warning"] = percent

    with open(config_file_path, "w") as f:
        json.dump(config_data, f, indent=4)


def generate_default(config_file) -> None:

    default_config = {
        "CPU": {"warning": 80, "critical": 90},
        "RAM": {"warning": 80, "critical": 90},
        "Disk": {"warning": 80, "critical": 90},
    }

    with open(config_file, "w") as f:
        json.dump(default_config, f, indent=4)


def get_file_path() -> Path:

    config_dir = (
        Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config")) / "updog"
    )
    config_dir.mkdir(parents=True, exist_ok=True)
    config_file_path = config_dir / "config.json"

    # If config file not exist, fill with default value
    if not config_file_path.exists():
        generate_default(config_file_path)

    return config_file_path


def get_config() -> dict:

    config_file_path = get_file_path()

    # Read from file
    with open(config_file_path, "r") as f:
        data = json.load(f)

    return data


def print_config() -> None:
    threshold = get_config()
    for key, value in threshold.items():
        print(f"- {key} ", end="")
        for key2, value2 in value.items():
            print(f"{key2.capitalize()}: {value2}% | ", end="")
        print("")
