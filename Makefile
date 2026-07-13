.PHONY: all build test vet lint clean

GO ?= go
BINARY ?= go-comic-converter

all: build

build:
	$(GO) build -o $(BINARY) .

test:
	$(GO) test -timeout 120s -count=1 ./...

vet:
	$(GO) vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
	rm -rf /tmp/go-comic-converter-*.tmp
