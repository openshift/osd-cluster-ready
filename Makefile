# Project specific values
IMAGE_NAME ?= osd-cluster-ready

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

include boilerplate/generated-includes.mk

.PHONY: build
build:
	GOOS=linux go build -mod=mod -ldflags="-s -w" -o ./bin/main main.go

.PHONY: deploy
deploy:
	hack/deploy.sh
