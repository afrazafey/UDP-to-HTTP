VERSION ?= 0.2.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -s -w -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Version=$(VERSION) -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Commit=$(COMMIT) -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Date=$(DATE)

.PHONY: frontend build test check linux
frontend:
	cd frontend && npm ci && npm run build
build: frontend
	mkdir -p build
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o build/udphttp-proxy ./cmd/udphttp-proxy
test:
	go test ./...
	cd frontend && npm test
check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	go test -race ./...
	cd frontend && npm run typecheck && npm test && npm run build
linux: frontend
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o build/udphttp-proxy-linux-amd64 ./cmd/udphttp-proxy
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o build/udphttp-proxy-linux-arm64 ./cmd/udphttp-proxy
