# Project specific values
IMAGE_NAME ?= osd-cluster-ready

# Prow scan image may lag; go.mod requires 1.25.10+ for govulncheck stdlib fixes.
export GOTOOLCHAIN=go1.25.11+auto

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

.PHONY: scan
scan:
	govulncheck ./...

.PHONY: deploy
deploy:
	hack/deploy.sh
