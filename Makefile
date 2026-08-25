# Project specific values
IMAGE_NAME ?= osd-cluster-ready

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

include boilerplate/generated-includes.mk

# Boilerplate ships golangci-lint built with Go 1.25; go.mod is 1.26 (k8s 0.36).
# Pin the linter language version until boilerplate ships a 1.26 binary.
.PHONY: lint
lint:
	${LINT_CONVENTION_DIR}/ensure.sh golangci-lint
	GOLANGCI_LINT_CACHE=${GOLANGCI_LINT_CACHE} golangci-lint run -c ${LINT_CONVENTION_DIR}/golangci.yml --go=1.25 ./...

.PHONY: build
build:
	GOOS=linux go build -mod=readonly -ldflags="-s -w" -o ./bin/main .

.PHONY: test
test:
	go test -mod=readonly -v ./...

.PHONY: scan
scan:
	govulncheck ./...

.PHONY: deploy
deploy:
	hack/deploy.sh
