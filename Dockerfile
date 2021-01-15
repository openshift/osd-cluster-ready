FROM fedora:latest

RUN yum install --assumeyes \
    jq \
    wget

ADD dockerbuild /root/build
RUN /root/build/install-oc.sh

ADD bin/main /root/main

