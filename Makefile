.PHONY: build fmt lint test validate check

build:
	go build ./cmd/acpa

fmt:
	gofmt -w cmd internal

lint:
	go vet ./...

test:
	go test ./...

validate:
	openspec validate bootstrap-acp-assistant-platform --strict

check: fmt lint test build validate
