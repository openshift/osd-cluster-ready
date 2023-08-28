FROM registry.ci.openshift.org/openshift/release:golang-1.19 as builder

WORKDIR /go/src/osd-cluster-ready
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM registry.access.redhat.com/ubi8/ubi-micro:8.8-5

COPY --from=builder /go/src/osd-cluster-ready/bin/main /root/
