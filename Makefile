IMAGE_URI ?= $(IMAGE_REPO)/$(IMAGE_ORG)/$(IMAGE_NAME)

# Project specific values
DOCKER_IMAGE_REGISTRY?=docker.io
QUAY_IMAGE_REGISTRY?=quay.io
IMAGE_REPOSITORY?=openshift-sre
IMAGE_NAME?=osd-cluster-ready
DOCKERFILE=./Dockerfile

# Podman by default, fall back to docker
CONTAINER_ENGINE=$(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)

# Gather commit number for Z and short SHA
COMMIT_NUMBER=$(shell git rev-list `git rev-list --parents HEAD | egrep "^[a-f0-9]{40}$$"`..HEAD --count)
CURRENT_COMMIT=$(shell git rev-parse --short=7 HEAD)

# Build container version
VERSION_MAJOR?=0
VERSION_MINOR?=1
CONTAINER_VERSION=$(VERSION_MAJOR).$(VERSION_MINOR).$(COMMIT_NUMBER)-$(CURRENT_COMMIT)

# Quay.io image
QUAY_IMG?=$(QUAY_IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):v$(CONTAINER_VERSION)
QUAY_IMAGE_URI=${QUAY_IMG}
QUAY_IMAGE_URI_LATEST=$(QUAY_IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest

# Docker image
DOCKER_IMG?=$(DOCKER_IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):v$(CONTAINER_VERSION)
DOCKER_IMAGE_URI=${DOCKER_IMG}
DOCKER_IMAGE_URI_LATEST=$(DOCKER_IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest

.PHONY: build
build:
	GOOS=linux go build -o ./bin/main main.go

.PHONY: docker-build
docker-build: build
	# Build and tag images for quay.io
	${CONTAINER_ENGINE} build . -f $(DOCKERFILE) -t $(QUAY_IMAGE_URI)
	${CONTAINER_ENGINE} tag $(QUAY_IMAGE_URI) $(QUAY_IMAGE_URI_LATEST)
	# Tag docker images
	${CONTAINER_ENGINE} tag $(QUAY_IMAGE_URI) $(DOCKER_IMAGE_URI)
	${CONTAINER_ENGINE} tag $(DOCKER_IMAGE_URI) $(DOCKER_IMAGE_URI_LATEST)

.PHONY: docker-push
docker-push:
	# Push Quay.io images
	${CONTAINER_ENGINE} push $(QUAY_IMAGE_URI)
	${CONTAINER_ENGINE} push $(QUAY_IMAGE_URI_LATEST)
	# Push Docker images
	# ${CONTAINER_ENGINE} push $(DOCKER_IMAGE_URI)
	# ${CONTAINER_ENGINE} push $(DOCKER_IMAGE_URI_LATEST)

.PHONY: deploy
deploy:
	hack/deploy.sh
