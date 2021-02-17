#!/bin/bash

if [ -z "$IMAGE_REPOSITORY" ]; then
  echo "Not set"
else
  echo "$IMAGE_REPOSITORY"
fi
# Gather commit number for Z and short SHA
COMMIT_NUMBER=$(git rev-list `git rev-list --parents HEAD | egrep "^[a-f0-9]{40}$"`..HEAD --count)
CURRENT_COMMIT=$(git rev-parse --short=7 HEAD)

# Build container version
VERSION_MAJOR=0
VERSION_MINOR=1
CONTAINER_VERSION="v$VERSION_MAJOR.$VERSION_MINOR.$COMMIT_NUMBER-$CURRENT_COMMIT"

TMP_MANIFEST=$(mktemp)
echo "Created $TMP_MANIFEST"
cat deploy/60-osd-ready.Job.yaml | sed "s/openshift-sre/${IMAGE_REPOSITORY}/" > $TMP_MANIFEST
sed -i "s/\/osd-cluster-ready/\/osd-cluster-ready:${CONTAINER_VERSION}/" $TMP_MANIFEST
sed -i "s/value: \"240\"/value: \"339860\"/" $TMP_MANIFEST
trap "rm -fr $TMP_MANIFEST" EXIT
cat $TMP_MANIFEST

oc delete job -n openshift-monitoring osd-cluster-ready

for manifest in $(ls deploy/ | grep -v "60-osd-ready.Job.yaml")
do
    oc apply -f deploy/${manifest}
done

oc apply -f $TMP_MANIFEST

# oc logs -f jobs/osd-cluster-ready -n openshift-monitoring
