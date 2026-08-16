BINARY := dist/kata-exporter
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet fmt cross clean
build:
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/kata-exporter

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

cross:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/kata-exporter-linux-amd64 ./cmd/kata-exporter
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/kata-exporter-linux-arm64 ./cmd/kata-exporter

clean:
	rm -r dist

