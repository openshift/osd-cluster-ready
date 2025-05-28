FROM quay.io/redhat-services-prod/openshift/boilerplate:image-v7.3.0 as builder

WORKDIR /go/src/osd-cluster-ready
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM registry.access.redhat.com/ubi8/ubi-micro:8.10-24
WORKDIR /

RUN mkdir /licenses
COPY --from=builder /go/src/osd-cluster-ready/LICENSE /licenses/.
COPY --from=builder /go/src/osd-cluster-ready/bin/main /osd-cluster-ready

USER 65532:65532
ENTRYPOINT ["/osd-cluster-ready"]
