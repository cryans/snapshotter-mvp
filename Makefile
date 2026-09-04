.PHONY: build run test clean

build:
	go build -o bin/snapshotter .

run: build
	./bin/snapshotter

test:
	go test -v ./...

clean:
	rm -rf bin/

