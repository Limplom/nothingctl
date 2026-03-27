.PHONY: build dist prep test clean

VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")

prep:
	cp ../debloat.json internal/data/debloat.json
	cp ../modules.json internal/data/modules.json

build: prep
	go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../nothingctl ./cmd/nothingctl/

dist: prep
	mkdir -p ../dist
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../dist/nothingctl-windows-amd64.exe ./cmd/nothingctl/
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../dist/nothingctl-linux-amd64     ./cmd/nothingctl/
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../dist/nothingctl-linux-arm64      ./cmd/nothingctl/
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../dist/nothingctl-darwin-arm64    ./cmd/nothingctl/
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../dist/nothingctl-darwin-amd64    ./cmd/nothingctl/

test: prep
	go test ./...

clean:
	rm -f ../nothingctl ../nothingctl.exe ../dist/nothingctl-*
	rm -f internal/data/debloat.json internal/data/modules.json
