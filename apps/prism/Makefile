.PHONY: build test lint install clean

APP := prism
GOFLAGS := -trimpath

build:
	go build $(GOFLAGS) -o ../../bin/runtime-$(APP) ./cmd/$(APP)

test:
	go test ./...

lint:
	golangci-lint run ./...

install:
	go install ./cmd/$(APP)

clean:
	rm -f ../../bin/runtime-$(APP)
