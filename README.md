# OSD Cluster Readiness Job

- [OSD Cluster Readiness Job](#osd-cluster-readiness-job)
  - [Deploying the Image](#deploying-the-image)
  - [Deploying the Job](#deploying-the-job)
  - [Tunables](#tunables)
    - [`MAX_CLUSTER_AGE_MINUTES`](#max_cluster_age_minutes)
    - [`CLEAN_CHECK_RUNS`](#clean_check_runs)
    - [`CLEAN_CHECK_INTERVAL_SECONDS`](#clean_check_interval_seconds)
    - [`FAILED_CHECK_INTERVAL_SECONDS`](#failed_check_interval_seconds)
- [TO DO](#to-do)

This job silences alerts while Day2 configuration is loaded onto a cluster at initial provisioning, allowing it to not page on-call SREs for normal operations within the cluster.

The silence initially takes effect for 1 hour.

We poll cluster health using [osde2e health checks](https://github.com/openshift/osde2e/blob/041355675304a7aa371b7fbeea313001036feb75/pkg/common/cluster/clusterutil.go#L211)
once a minute (this is [configurable](#failed_check_interval_seconds)),
until they all report healthy 20 times in a row ([configurable](#clean_check_runs))
on 30s intervals ([configurable](#clean_check_interval_seconds)).
By default, we will clear any active silence and exit successfully if the cluster is (or becomes) more than two hours old ([configurable](#max_cluster_age_minutes)).

If the silence expires while health checks are failing, we reinstate it.
(This means it is theoretically possible for alerts to fire for up to one minute if the silence expires right after a health check fails. [FIXME](#to-do).)

## Deploying the Image

```
make build
make docker-build
make docker-push
```

This builds the binary for linux, builds the docker image (which requires the binary to be built externally as of right now) and then pushes the updated image to quay.

If you wish to push to a specific repository, org, or image name, you may override the `IMAGE_REPO`, `IMAGE_ORG`, or `IMAGE_NAME` variables, respectively, when invoking the `docker-build` and `docker-push` targets.
For example, for development purposes, you may wish to `export IMAGE_ORG=my_quay_namespace`.

## Deploying the Job

Deploy each of the manifests in the [deploy/](deploy) folder in alphanumeric order.

If you are overriding any of the `IMAGE_*` variables for development purposes, be sure to (temporarily) edit the [Job](deploy/60-osd-ready.Job.yaml), setting the `image` appropriately.

You can iterate by deleting the Job (which will delete its Pod) and recreating it.

## Tunables
The following environment variables can be set in the container, e.g. by editing the [Job](deploy/60-osd-ready.Job.yaml) to include them in `spec.template.spec.containers[0].env`.

Remember that the values must be strings; so numeric values must be quoted.

### `MAX_CLUSTER_AGE_MINUTES`
The maximum age of the cluster, in minutes, after which we will clear any silences and exit "successfully".

**Default:** `"120"` (two hours)

### `CLEAN_CHECK_RUNS`
The number of consecutive health checks that must succeed before we declare the cluster truly healthy.

**Default:** `"20"`

### `CLEAN_CHECK_INTERVAL_SECONDS`
The number of seconds to sleep between successful health checks.
Once the cluster is truly healthy, you can expect the job to succeed after an interval of roughly:

`CLEAN_CHECK_RUNS` x (`CLEAN_CHECK_INTERVAL_SECONDS` + (time to run one iteration of health checks)) seconds

**Default:** `"30"` (seconds)

### `FAILED_CHECK_INTERVAL_SECONDS`
The number of seconds to sleep after a failed health check, before rechecking.

**Default:** `"60"` (one minute)

# TO DO

[x] Look for existing active silences before creating a new one
[x] Implement _actual_ healthchecks (steal them from osde2e) to determine cluster stability
[ ] Find if there's a better and more secure way to talk to the alertmanager API using oauth and serviceaccount tokens.  
[ ] Make the default silence expiry shorter; and extend it when health checks fail.