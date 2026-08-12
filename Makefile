.DEFAULT_GOAL := check

.PHONY: check check-layout fmt generate test test-race vet

check: check-layout vet test

check-layout:
	./scripts/check-layout.sh

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

generate:
	go generate .

vet:
	go vet ./...

test:
	go test ./... -count=1 -timeout 240s

test-race:
	go test -race ./... -count=1 -timeout 600s
