from pathlib import Path
import json
import os


# Default config
def generate_default(config_file) -> None:

    default_config = {
        "cpu_threshold": 80,
        "ram_threshold": 80,
        "disk_threshold": 80,
    }

    with open(config_file, "w") as f:
        json.dump(default_config, f, indent=4)

    return


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

    print(f"Data: {data}")
    print(f"Data type: {type(data)}")

    return data
