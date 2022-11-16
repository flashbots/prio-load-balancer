.PHONY: all v build test clean lint cover cover-html docker-image

VERSION := $(shell git describe --tags --always --dirty="-dev")

all: clean build

v:
	@echo "Version: ${VERSION}"

build:
	go build -trimpath -ldflags "-s -X main.version=${VERSION}" -v -o prio-load-balancer main.go

clean:
	rm -rf prio-load-balancer build/

test:
	go test ./...

lint:
	gofmt -d ./
	go vet ./...
	staticcheck ./...

lint-strict: lint
	gofumpt -d -extra .
	golangci-lint run

cover:
	go test -coverprofile=/tmp/go-prio-lb.cover.tmp ./...
	go tool cover -func /tmp/go-prio-lb.cover.tmp
	unlink /tmp/go-prio-lb.cover.tmp

cover-html:
	go test -coverprofile=/tmp/go-prio-lb.cover.tmp ./...
	go tool cover -html=/tmp/go-prio-lb.cover.tmp
	unlink /tmp/go-prio-lb.cover.tmp

docker-image:
	DOCKER_BUILDKIT=1 docker build --platform linux/amd64 --build-arg VERSION=${VERSION} . -t prio-load-balancer
