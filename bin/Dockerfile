FROM registry.ci.openshift.org/openshift/release:golang-1.17 as builder

WORKDIR /go/src/osd-cluster-ready
COPY go.mod go.sum ./
RUN go mod download
COPY boilerplate boilerplate
COPY main.go Makefile .
RUN make go-build

FROM registry.access.redhat.com/ubi8/ubi-micro:latest

COPY --from=builder /go/src/osd-cluster-ready/bin/osd-cluster-ready /
