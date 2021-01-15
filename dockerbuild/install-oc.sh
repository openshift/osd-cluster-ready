#!/bin/bash -e

ocv4client=openshift-client-linux-4.6.9.tar.gz

mkdir /usr/local/oc
pushd /usr/local/oc
wget -q https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.6.9/${ocv4client}
tar xzvf ${ocv4client}
rm ${ocv4client}
ln -s /usr/local/oc/oc /usr/local/bin/oc
oc completion bash >  /etc/bash_completion.d/oc
popd
