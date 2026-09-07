.PHONY: build run test clean fmt fmt-check vet tidy cover cover-report check demo

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

# Code coverage profile + per-function text report (CI uploads coverage.out).
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Visual, color-coded HTML report (per-file and per-line view). Requires the
# coverage.out produced by `make cover`. CI uploads coverage.html as an artifact.
cover-report:
	go tool cover -html=coverage.out -o coverage.html

# Regenerate the README demo GIF from the committed vhs tape (demo.tape).
# Requires vhs and ffmpeg on $PATH; see README.md "Demo".
demo: build
	PATH="$(CURDIR)/bin:$$PATH" vhs demo.tape

# Local analogue of the CI gate.
check: fmt-check vet test
