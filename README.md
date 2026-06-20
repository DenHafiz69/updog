# Updog - A CLI system monitoring and alert tool

## Features



## Installation

```bash
git clone https://github.com/DenHafiz69/updog.git
cd updog
uv venv
uv pip install -r requirements.txt
```

## Usage

### Configuration

* Print config to terminal. It would generate one if it does not exist yet.
```bash
updog config
	
```
* Set a specific value for a certain threshold in config.
```bash
updog config --set-critical cpu 95 # Set critical threshold for cpu to 95%
updog config --set-warning ram 85 # Set warning threshold for ram to 85%
```

### Check Status

* Print status for CPU, RAM, and Disk to terminal.
```bash
updog status
```
* Only print specified metric (i.e CPU only) to terminal.
```bash
updog status cpu
```
