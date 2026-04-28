.PHONY: all build coverage dev help lint test vet

COVERAGE_THRESHOLD := 75.0

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make dev    Start the Nix development shell' \
		'  make build  Build deps2flake into target/' \
		'  make test   Run Go tests' \
		'  make coverage  Run Go tests with coverage output in target/' \
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
	go test -race ./...

coverage:
	mkdir -p target
	go test -race -coverprofile=target/coverage.out ./...
	go tool cover -func=target/coverage.out
	@total=$$(go tool cover -func=target/coverage.out | awk '/^total:/ { sub(/%/, "", $$3); print $$3 }'); \
	awk -v total="$$total" -v threshold="$(COVERAGE_THRESHOLD)" 'BEGIN { \
		if (total + 0 < threshold + 0) { \
			printf "coverage %.1f%% is below required %.1f%%\n", total, threshold; \
			exit 1; \
		} \
		printf "coverage %.1f%% meets required %.1f%%\n", total, threshold; \
	}'

lint:
	golangci-lint run ./...

vet:
	go vet ./...
