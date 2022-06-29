# Project specific values
IMAGE_NAME ?= osd-cluster-ready

# GOLANGCI_LINT_CACHE needs to be set to a directory which is writeable
# Relevant issue - https://github.com/golangci/golangci-lint/issues/734
GOLANGCI_LINT_CACHE ?= /tmp/golangci-cache

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

include boilerplate/generated-includes.mk

.PHONY: build
build:
	GOOS=linux go build -mod=readonly -ldflags="-s -w" -o ./bin/main main.go

.PHONY: test
test:
	go test -mod=readonly -v ./...

.PHONY: deploy
deploy:
	hack/deploy.sh

.PHONY: lint
lint:
	GOLANGCI_LINT_CACHE=${GOLANGCI_LINT_CACHE} golangci-lint run --mod=readonly ./...
