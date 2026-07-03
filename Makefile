.PHONY: build test

build:
	go build -o bin/rm ./cmd/rm

test:
	go test ./...
