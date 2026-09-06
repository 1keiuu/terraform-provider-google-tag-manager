.DEFAULT_GOAL := build

.PHONY: build test test-race check generate update-discovery docs

build:
	go build -o terraform-provider-google-tag-manager .

test:
	go test ./...

test-race:
	go test -race ./...

generate:
	go generate ./...

docs: generate

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./...
	go generate ./...
	git diff --exit-code -- docs

update-discovery:
	./scripts/update-discovery.sh
