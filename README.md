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

## Deploying

### Build the Image

```bash
make docker-build
make docker-push
```

This builds the docker image, and then pushes the image to a repository.

If you wish to push to a specific registry, repository, or image name, you may override the `IMAGE_REGISTRY`, `IMAGE_REPOSITORY`, or `IMAGE_NAME` variables, respectively, when invoking the `docker-build` and `docker-push` targets.
For example, for development purposes, you may wish to `export IMAGE_REPOSITORY=my_quay_user`.
See the [boilerplate/project.mk](boilerplate/project.mk) for the default values.

### Deploy
**NOTE:** In OSD, this program is managed by a Job deployed to the cluster from Hive via a SelectorSyncSet maintained in [managed-cluster-config](https://github.com/openshift/managed-cluster-config/tree/01332ca90e15cd9a0d67cdcc596f538fa8869dbb/deploy/osd-cluster-ready).
In order to prevent overwrites during testing, you must [pause hive syncing](https://github.com/openshift/ops-sop/blob/master/v4/knowledge_base/pause-syncset.md).

```bash
make deploy
```

This will do the following on your currently logged-in cluster. **NOTE:** You must have elevated permissions.
- Delete any existing `osd-cluster-ready` Job.
- Deploy each of the manifests in the [deploy/](deploy) folder in alphanumeric order, except the [Job](deploy/60-osd-ready.Job.yaml) itself.
- Create a temporary Job manifest, overriding the `image` using the `IMAGE_*` variables described above, and deploy it.
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

- [ ] Make [tunables](#tunables) configurable via `make deploy`.
