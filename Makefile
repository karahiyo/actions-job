SHELL=/bin/bash

.PHONY: help test lint lint-fix
help:
	cat Makefile

test:
	go test -v -race $(option) ./...

lint:
	golangci-lint run -v

lint-fix:
	golangci-lint run -v --fix
	fieldalignment -fix ./... # golangci-lint does not support fieldalignment with --fix yet

.PHONY: helper/list-all-queued-workflows helper/stop-all-queued-workflows
helper/list-all-queued-workflows:
	gh run list --limit 100 --jq ".[] | select (.status == \"queued\" ) | .databaseId" --json databaseId,status

helper/stop-all-queued-workflows:
	gh run list --limit 100 --jq ".[] | select (.status == \"queued\" ) | .databaseId" --json databaseId,status | xargs -n 1 gh run cancel
