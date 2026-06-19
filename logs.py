def get_logs(args, limit=5):
    # Use to print logs. Default is return 5 latest logs
    # If user use "--limit 2" flag, return latest 2 logs

    if "--limit" in args:
        limit = int(args[args.index("--limit") + 1])

    with open("updog.log", "r") as f:
        lines = f.readlines()

    for line in lines[-limit:]:
        print(line, end="")
