# AGENTS.md

## Pre-Push Checks (Required)

Before committing and pushing any Go changes to remote, always run:

1. `go vet ./...`
2. `staticcheck ./...`
3. `gosec ./...`
4. `go test -race ./...`

Fix any reported issues (or explicitly justify and annotate false positives) before pushing.
