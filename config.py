from pathlib import Path, PosixPath
import json
import os


# Default config
def get_default() -> dict:

    default_config = {
        "cpu_threshold": 80,
        "ram_threshold": 80,
    }

    return default_config


def get_file_path() -> Path:

    config_dir = (
        Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config")) / "updog"
    )
    config_dir.mkdir(parents=True, exist_ok=True)
    config_file = config_dir / "config.json"

    return config_file


def get_config():

    config_file = get_file_path()
    default_config = get_default()

    # Write to file
    with open(config_file, "w") as f:
        json.dump(default_config, f, indent=4)

    # Read from file
    with open(config_file, "r") as f:
        data = json.load(f)

    print(f"Data: {data}")
