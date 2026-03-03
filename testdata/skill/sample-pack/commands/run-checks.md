---
description: Run pre-commit checks including linting, formatting, and tests
---
# Run Checks

Run the following checks before committing:

1. `go fmt ./...`
2. `go vet ./...`
3. `go test ./...`

Report any failures with the specific file and error message.
