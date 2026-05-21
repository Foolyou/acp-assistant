.PHONY: build console-build console-test console-smoke fmt lint test validate check

build:
	go build ./cmd/acpa

console-build:
	npm run console:build

console-test:
	npm run console:test

console-smoke:
	npm run console:smoke

fmt:
	gofmt -w cmd internal

lint:
	go vet ./...

test:
	go test ./...

validate:
	openspec validate bootstrap-acp-assistant-platform --strict

check: fmt lint console-build console-test test build validate
