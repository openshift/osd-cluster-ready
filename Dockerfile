FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

RUN microdnf install \
    gzip \
    jq \
    tar \
    wget

ADD dockerbuild /root/build
RUN /root/build/install-oc.sh

ADD bin/main /root/main
