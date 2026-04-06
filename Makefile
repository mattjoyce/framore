.PHONY: build test lint sec fmt check

build:
	go build -o bin/framore ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

sec:
	gosec ./...

fmt:
	gofmt -w .
	goimports -w .

check: fmt lint sec test
