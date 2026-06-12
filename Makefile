.PHONY: build test run clean install

build:
    go build -o bin/gorelay ./examples/basic

test:
    go test -v ./...

run:
    go run ./examples/basic/main.go

clean:
    rm -rf bin/
    rm -f relay.db

install:
    go install ./...

docker-build:
    docker build -t gorelay:latest .

deps:
    go mod download
    go mod tidy

lint:
    golangci-lint run

bench:
    go test -bench=. -benchmem ./...