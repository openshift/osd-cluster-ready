include boilerplate/project.mk
include boilerplate/standard.mk

SHELL := /usr/bin/env bash

# Used by deploy.sh
export IMAGE_URI_VERSION

.PHONY: deploy
deploy:
	hack/deploy.sh
