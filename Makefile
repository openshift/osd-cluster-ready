IMAGE_REPO ?= quay.io
IMAGE_ORG ?= openshift-sre
IMAGE_NAME ?= osd-cluster-ready
IMAGE_URI ?= $(IMAGE_REPO)/$(IMAGE_ORG)/$(IMAGE_NAME)

.PHONY: build
build:
	GOOS=linux go build -o ./bin/main main.go

.PHONY: docker-build
docker-build:
	docker build . -t $(IMAGE_URI)

.PHONY: docker-push
docker-push:
	docker push $(IMAGE_URI)