# Updog - A CLI system monitoring and alert tool

## Features



## Installation

```bash
git clone https://github.com/DenHafiz69/updog.git
cd updog
uv pip install -e .
```

## Usage

### Configuration

* Print config to terminal. It would generate one if it does not exist yet.
```bash
updog config
```
* Set a specific value for a certain threshold in config.
```bash
updog config --default # Reset the config to default values
updog config --set-warning ram 85 # Set warning threshold for ram to 85%
updog config --set-critical cpu 95 # Set critical threshold for cpu to 95%
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
* Check and print status to logs. Does not print or return anything. Should be used with a cronjob(?)
```bash
updog check
updog check 10 # Repeat the checking 10 times
```

### Print Logs

* Print logs to terminal. Can specify how many lines of logs do you want.
```bash
updog logs # Print 5 lines of logs to terminal
updog logs --limit 2
```
