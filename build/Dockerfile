FROM quay.io/redhat-services-prod/openshift/boilerplate:image-v7.3.0 as builder

WORKDIR /go/src/osd-cluster-ready
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM registry.access.redhat.com/ubi8/ubi-micro:8.10-13

COPY --from=builder /go/src/osd-cluster-ready/bin/main /root/
