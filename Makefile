VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build run test vet fmt generate docker clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/timetools ./cmd/timetools

run:
	TT_BASE_URL=localhost:8080 go run ./cmd/timetools

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Regenerates the zone table; needs a host with tzdata installed.
generate:
	go run ./gen/zones > internal/tz/zones_gen.go

docker:
	docker build --build-arg VERSION=$(VERSION) -t timetools .

clean:
	rm -rf bin
