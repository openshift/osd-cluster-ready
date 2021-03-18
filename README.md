# OSD Cluster Readiness

- [OSD Cluster Readiness](#osd-cluster-readiness)
  - [Deploying](#deploying)
    - [Build the Image](#build-the-image)
    - [Deploy](#deploy)
    - [Example](#example)
  - [Tunables](#tunables)
    - [`CLEAN_CHECK_RUNS`](#clean_check_runs)
    - [`CLEAN_CHECK_INTERVAL_SECONDS`](#clean_check_interval_seconds)
    - [`FAILED_CHECK_INTERVAL_SECONDS`](#failed_check_interval_seconds)
  - [Keeping up with osde2e](#keeping-up-with-osde2e)
- [TO DO](#to-do)

This program polls cluster health using [osde2e health checks](https://github.com/openshift/osde2e/blob/041355675304a7aa371b7fbeea313001036feb75/pkg/common/cluster/clusterutil.go#L211)
once a minute (this is [configurable](#failed_check_interval_seconds)),
until they all report healthy 20 times in a row ([configurable](#clean_check_runs))
on 30s intervals ([configurable](#clean_check_interval_seconds)).
By default, we will exit successfully if the cluster is (or becomes) more than two hours old ([configurable](#max_cluster_age_minutes)).

## Deploying

### Build the Image

```
make image-build
make image-push
```

This builds the binary for linux, builds the docker image, and then pushes the image to a repository.

If you wish to push to a specific registry, repository, or image name, you may override the `IMAGE_REGISTRY`, `IMAGE_USER`, or `IMAGE_NAME` variables, respectively, when invoking the `image-build` and `image-push` targets.
For example, for development purposes, you may wish to `export IMAGE_USER=my_quay_user`.
See the [Makefile](Makefile) for the default values.

### Deploy
**NOTE:** In OSD, this program is managed by [configure-alertmanager-operator](https://github.com/openshift/configure-alertmanager-operator) via a Job it [defines internally](https://github.com/openshift/configure-alertmanager-operator/blob/master/pkg/readiness/defs/osd-cluster-ready.Job.yaml).
In order to test locally, that needs to be disabled. (**TODO: How?**)

```
make deploy
```

This will do the following on your currently logged-in cluster. **NOTE:** You must have elevated permissions.
- Delete any existing `osd-cluster-ready` Job.
- Deploy each of the manifests in the [deploy/](deploy) folder in alphanumeric order, except the [Job](deploy/60-osd-ready.Job.yaml) itself.
- Create a temporary Job manifest with the following overrides, and deploy it:
  - The `image` is set using any of the `IMAGE_*` overrides described above.
  - The [`MAX_CLUSTER_AGE_MINUTES` environment variable](#max_cluster_age_minutes) is set to a high value to prevent the job from exiting early. ([FIXME: make this configurable](#to-do).)
- Wait for the Job's Pod to start and follow its logs.

In addition to the `IMAGE_*` overrides, `make deploy` will also observe the following environment variables:
- `JOB_ONLY`: If set (to any `true`-ish value), only deploy the overridden Job manifest.
  Use this to streamline the deployment process if the other manifests (RBAC, etc.) are already deployed and unchanged.
- `DRY_RUN`: Don't actually do anything to the cluster; just print the overridden Job manifest and the commands that _would_ have been run.

### Example

Build, push to, and deploy from my personal namespace, `i_am_a_docker`, in the docker.io registry, skipping RBAC manifests, and first doing a dry run:

```
# Set these in the environment to save passing them to each `make` command.
export IMAGE_REGISTRY=docker.io
export IMAGE_USER=i_am_a_docker

make image-build image-push

# Do a deploy dry run first
make JOB_ONLY=1 DRY_RUN=1 deploy

# Now deploy for real
make JOB_ONLY=1 deploy
```

## Tunables
The following environment variables can be set in the container, e.g. by editing the [Job](deploy/60-osd-ready.Job.yaml) to include them in `spec.template.spec.containers[0].env`.

Remember that the values must be strings; so numeric values must be quoted.

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

## Keeping up with osde2e
This code runs health checks via a module dependency on `github.com/openshift/osde2e`.
That dependency is pinned to a specific commit in [go.mod](go.mod).
That commit must be modified manually to pick up changes in osde2e.
An easy way to bump to the latest commit is to run:

```
go get -u github.com/openshift/osde2e
```

Don't forget to [build](#deploying-the-image) and [test](#deploying-the-job) with the updated dependency before committing!

# TO DO

- [x] Implement _actual_ healthchecks (steal them from osde2e) to determine cluster stability
- [ ] Make [tunables](#tunables) configurable via `make deploy`.
