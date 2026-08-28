# Every repeatable command for this repo lives here.
#
# This file is the single source of truth for how the application is set up,
# built, run and checked. If you find yourself typing a multi-part command
# twice — a build with particular flags, a tool invocation with a path prefix,
# a test run with particular arguments — add a target for it here rather than
# keeping it in your shell history or a README. Anything documented elsewhere
# should point at a target in this file instead of repeating the command, so
# there is one place to change when a command changes.

# asdf gives every Go version its own GOPATH, so the Wails CLI has to be
# installed once per Go toolchain and is found under the *current* one. Going
# through `go env GOPATH` rather than PATH is what keeps this correct after a
# Go upgrade; `make tools` is what fixes it when the binary is not there yet.
WAILS := $(shell go env GOPATH)/bin/wails
APP   := build/bin/scriptorium.app

.DEFAULT_GOAL := help

## help: list the available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  make %-12s %s\n", $$1, $$2}'

# --- setup ---------------------------------------------------------------

## setup: install every tool and dependency needed to build the app
setup: tools deps

## tools: install the Wails CLI for the current Go toolchain
tools:
	go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
	@command -v asdf >/dev/null 2>&1 && asdf reshim golang || true

# Fails with the fix rather than with "no such file or directory". This is the
# first thing that breaks after a Go upgrade, because the new toolchain's GOPATH
# starts out empty.
require-wails:
	@test -x $(WAILS) || { \
		echo "Wails CLI not installed for $$(go version | cut -d' ' -f3)."; \
		echo "Expected it at $(WAILS)"; \
		echo "Run: make tools"; \
		exit 1; \
	}

## deps: download Go modules and frontend packages
deps:
	go mod download
	cd frontend && npm install

## doctor: check the machine has what Wails needs
doctor: require-wails
	$(WAILS) doctor

# --- running -------------------------------------------------------------

## dev: run the app with live reload
dev: require-wails
	$(WAILS) dev

## build: produce the packaged macOS application
build: require-wails
	$(WAILS) build

## run: build the app and open it
run: build
	open $(APP)

## headless: serve the interface over loopback for manual verification
headless:
	@go run ./cmd/headless

# --- assets --------------------------------------------------------------

# The output is committed, so go build and go test work without npm installed.
## assets: rebuild the Tailwind bundle and re-vendor htmx into internal/ui/static
assets:
	cd frontend && npm run build

# --- checks --------------------------------------------------------------

## check: everything CI would run — formatting, vet, and the full test suite
check: fmt-check vet test

## test: run the full test suite
test:
	go test ./...

## test-package: run tests for one Go package (make test-package PACKAGE=./internal/workspace)
test-package:
	go test $(PACKAGE)

## vet: run go vet
vet:
	go vet ./...

## fmt: format the Go sources in place
fmt:
	gofmt -w .

## fmt-check: fail if any Go source is unformatted
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "unformatted files:"; echo "$$unformatted"; exit 1; \
	fi

# --- housekeeping --------------------------------------------------------

## clean: remove build output
clean:
	rm -rf build/bin

.PHONY: help setup tools require-wails deps doctor dev build run headless assets check test vet fmt fmt-check clean
