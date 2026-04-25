.PHONY: all build dev help lint test vet

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make dev    Start the Nix development shell' \
		'  make build  Build deps2flake into target/' \
		'  make test   Run Go tests' \
		'  make lint   Run golangci-lint' \
		'  make vet    Run go vet' \
		'  make all    Run build, test, lint, and vet'

all: build test lint vet

dev:
	nix develop

build:
	mkdir -p target
	go build -o target/deps2flake ./cmd/deps2flake

test:
	go test ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...
