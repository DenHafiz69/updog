from pathlib import Path
import json
import os


def generate_default(config_file) -> None:

    default_config = {
        "CPU": [{"warning": 80, "critical": 90}],
        "RAM": [{"warning": 80, "critical": 90}],
        "Disk": [{"warning": 80, "critical": 90}],
    }

    with open(config_file, "w") as f:
        json.dump(default_config, f, indent=4)


def get_file_path() -> Path:

    config_dir = (
        Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config")) / "updog"
    )
    config_dir.mkdir(parents=True, exist_ok=True)
    config_file = config_dir / "config.json"

    return config_file


def get_config() -> dict:

    config_file = get_file_path()

    # If config file not exist, fill with default value
    if not config_file.exists():
        generate_default(config_file)

    # Read from file
    with open(config_file, "r") as f:
        data = json.load(f)

    return data


def print_config() -> None:
    threshold = get_config()
    for key, value in threshold.items():
        print(f"- {key} ", end="")
        for key2, value2 in value[0].items():
            print(f"{key2.capitalize()}: {value2}% | ", end="")
        print("")
