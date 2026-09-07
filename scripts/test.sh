#!/usr/bin/env bash
# Pure-Go unit tests (no CGO needed). Fyne ui/ package is excluded here.
set -e
CGO_ENABLED=0 go test ./internal/...
