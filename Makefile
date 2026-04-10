GO ?= go

.PHONY: test vet build linux smoke
test:
	$(GO) test -race -cover ./...
	python3 -m unittest discover -s scripts -p 'test_*.py'

vet:
	$(GO) vet ./...
	test -z "$$($(GO) fmt ./...)"

build:
	$(GO) build -trimpath -o bin/watchhouse ./cmd/watchhouse

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -o bin/watchhouse-linux-amd64 ./cmd/watchhouse
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -o bin/watchhouse-linux-arm64 ./cmd/watchhouse

smoke:
	GO=$(GO) sh scripts/linux-smoke.sh
