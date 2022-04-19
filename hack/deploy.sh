#!/bin/bash -e

usage() {
  cat <<EOF
Usage: $0
Environment:
  IMAGE_URI_VERSION (required): E.g. quay.io/my_repo/osd-cluster-ready:0.1.38-614bf59
  JOB_ONLY (optional): If set, only deploy the Job manifest (skip RBAC etc.).
  DRY_RUN (optional): If set, don't actually deploy anything, just print what would have happened.
EOF
  exit -1
}

maybe() {
  echo "+ $@"
  if [[ -z "$DRY_RUN" ]]; then
    $@
  fi
}

if [[ -z "$IMAGE_URI_VERSION" ]]; then
  echo "IMAGE_URI_VERSION not set"
  usage
fi

TMP_MANIFEST=$(mktemp -t osd-cluster-ready-Job.XXXXX.yaml)
trap "rm -fr $TMP_MANIFEST" EXIT
sed "s,\(^ *image: \).*,\1${IMAGE_URI_VERSION}," deploy/60-osd-ready.Job.yaml > $TMP_MANIFEST
echo "===== $TMP_MANIFEST ====="
cat $TMP_MANIFEST
echo "========================="

# In case the job is already deleted, don't let -e fail this, and don't wait
# for the pod to go away.
WAIT_FOR_POD=yes
maybe oc delete job -n openshift-monitoring osd-cluster-ready || WAIT_FOR_POD=no

if [[ -z "$JOB_ONLY" ]]; then
  echo "Deploying all the things. Set JOB_ONLY=1 to deploy only the Job."
  for manifest in $(ls deploy/ | grep -v "60-osd-ready.Job.yaml")
  do
      maybe oc apply -f deploy/${manifest}
  done
else
  echo "Deploying only the Job. To redeploy RBAC etc., unset JOB_ONLY."
fi

# Before deploying the new job, make sure the pod from the old one is gone
if [[ $WAIT_FOR_POD == "yes" ]]; then
  # FIXME: This can fail for two reasons:
  # - Timeout, in which case we want to blow up.
  # - The pod already disappeared, in which case we want to proceed.
  # Scraping the output is icky. For now, just ignore errors.
  maybe oc wait --for=delete pod -n openshift-monitoring -l job-name=osd-cluster-ready --timeout=30s || true
fi

maybe oc create -f $TMP_MANIFEST

if [[ -z "$DRY_RUN" ]]; then
  POD=$(oc get po -n openshift-monitoring -l job-name=osd-cluster-ready -o name)
else
  POD=osd-cluster-ready-XXXXX
fi

maybe oc wait --for=condition=Ready -n openshift-monitoring $POD --timeout=15s
if [[ $? -eq 0 ]]; then
  maybe oc logs -f jobs/osd-cluster-ready -n openshift-monitoring
fi
