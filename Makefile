.PHONY: build run test clean fmt fmt-check vet tidy cover check

build:
	go build -o bin/snapshotter .

run: build
	./bin/snapshotter

test:
	go test -v ./...

clean:
	rm -rf bin/

# Reformat all tracked Go source in place.
fmt:
	gofmt -w $$(git ls-files '*.go')

# Gate: fail if any tracked Go source is not gofmt-clean.
fmt-check:
	@files="$$(gofmt -l $$(git ls-files '*.go'))"; \
	if [ -n "$$files" ]; then \
		echo "gofmt needed in:"; \
		echo "$$files"; \
		exit 1; \
	fi; \
	echo "gofmt: clean"

vet:
	go vet ./...

tidy:
	go mod tidy

# Code coverage profile + per-function report (CI uploads coverage.out).
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Local analogue of the CI gate.
check: fmt-check vet test
