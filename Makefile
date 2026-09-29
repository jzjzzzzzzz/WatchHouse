GO ?= go

.PHONY: test vet build linux smoke crash docker-ports-integration audit-history
test:
	$(GO) test -race -cover ./...
	python3 -m unittest discover -s scripts -p 'test_*.py'

vet:
	$(GO) vet ./...
	test -z "$$($(GO) fmt ./...)"

build:
	$(GO) build -trimpath -o bin/watchhouse ./cmd/watchhouse
	$(GO) build -trimpath -o bin/watchhouse-control ./cmd/watchhouse-control

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -o bin/watchhouse-linux-amd64 ./cmd/watchhouse
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -o bin/watchhouse-linux-arm64 ./cmd/watchhouse
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -o bin/watchhouse-control-linux-amd64 ./cmd/watchhouse-control
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -o bin/watchhouse-control-linux-arm64 ./cmd/watchhouse-control

smoke:
	GO=$(GO) sh scripts/linux-smoke.sh

crash: build
	python3 scripts/spool-crash.py

docker-ports-integration:
	GO=$(GO) python3 scripts/docker-ports-integration.py

audit-history:
	python3 scripts/audit-history.py --require-complete
