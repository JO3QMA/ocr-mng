.PHONY: build lint test docker ci check-tests

COVERAGE_MIN ?= 20

build:
	go build -o bin/rm ./cmd/rm

lint:
	golangci-lint run ./...

check-tests:
	@for pkg in $$(go list ./internal/...); do \
		dir=$$(go list -f '{{.Dir}}' $$pkg); \
		if ! ls $$dir/*_test.go >/dev/null 2>&1; then \
			echo "missing _test.go in $$pkg"; exit 1; \
		fi; \
	done

test: check-tests
	go test ./internal/... -coverprofile=coverage.out
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/{gsub(/%/,""); print $$3}'); \
	awk -v t="$$total" -v min="$(COVERAGE_MIN)" 'BEGIN { if (t+0 < min+0) { printf "coverage %s%% < %s%%\n", t, min; exit 1 } }'

docker:
	docker build --platform linux/amd64 -t ocr-mng:local .

ci: lint test docker
