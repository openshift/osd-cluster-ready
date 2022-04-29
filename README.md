# OSD Cluster Readiness

This program polls cluster health using [osde2e health checks](https://github.com/openshift/osde2e/blob/6c77e8b0df7238ce7d5ae199f1628bf36353eb16/pkg/common/cluster/clusterutil.go#L394)
every 60 seconds on 30 second intervals, until it reports healthy 20 times in a row. See [tunables](#tunables) for adjusting these values.

In OSD, this program is managed by a Job deployed to the cluster from Hive via a SelectorSyncSet maintained in [managed-cluster-config](https://github.com/openshift/managed-cluster-config/tree/master/deploy/osd-cluster-ready).

## Local Development

This repo subscribes to the `osd-container-image` boilerplate convention. Deatiled information about its usage and make targets are available in its [README.md](./boilerplate/openshift/osd-container-image/README.md)

### Keeping up with osde2e

Bugs and corresponding fixes to health checks live in [github.com/openshift/osde2e](https://github.com/openshift/osde2e). That dependency is pinned to a specific commit in [go.mod](go.mod) and must be modified manually to pick up changes in osde2e. To update to the latest commit of osde2e, run:

```bash
go get -u github.com/openshift/osde2e
```

### Build

Without any override variables, the following make targets attempt to build and push and image to `quay.io/app-sre/osd-cluster-ready`. To build and push the image to a personal Quay repository, additionally override the following variables:

```bash
make osd-container-image-build IMAGE_REPOSITORY=myquayrepository
make osd-container-image-push IMAGE_REPOSITORY=myquayrepository REGISTRY_USER=myquayuser REGISTRY_TOKEN=myquaytoken
```

### Deploy

When you are satisfied with the built image, it can be deployed to the current active cluster context via:

```bash
make deploy IMAGE_URI_VERSION="quay.io/myquayrepository/vX.Y.Z"
```

This will do the following on your currently logged-in cluster. **NOTE:** You must have elevated permissions.

- Delete any existing `osd-cluster-ready` Job.
- Deploy each of the manifests in the [deploy/](deploy) folder in alphanumeric order, except the [Job](deploy/60-osd-ready.Job.yaml) itself.
- Create a temporary Job manifest, overriding the `image` using the `IMAGE_*` variables described above, and deploy it.
- Wait for the Job's Pod to start and follow its logs.

In addition to the `IMAGE_URI_VERSION` override, `make deploy` will also observe the following environment variables:

- `JOB_ONLY`: If set (to any `true`-ish value), only deploy the overridden Job manifest.
  Use this to streamline the deployment process if the other manifests (RBAC, etc.) are already deployed and unchanged.
- `DRY_RUN`: Don't actually do anything to the cluster; just print the overridden Job manifest and the commands that _would_ have been run.

## Tunables

The following environment variables can be set in the container, e.g. by editing the [Job](deploy/60-osd-ready.Job.yaml) to include them in `spec.template.spec.containers[0].env`.

Remember that the values must be strings; so numeric values must be quoted.

| Environment Variable | Purpose | Default |
|---|---|---|
| `CLEAN_CHECK_RUNS` | The number of consecutive health checks that must succeed before we declare the cluster truly healthy. | `"20"` |
| `CLEAN_CHECK_INTERVAL_SECONDS` | The number of seconds to sleep between successful health checks. | `"30"` |
| `FAILED_CHECK_INTERVAL_SECONDS` | The number of seconds to sleep after a failed health check, before rechecking. | `"60"` |

Once the cluster is truly healthy, you can expect the job to succeed after an interval of roughly:

`CLEAN_CHECK_RUNS` x (`CLEAN_CHECK_INTERVAL_SECONDS` + (time to run one iteration of health checks)) seconds

## TO DO

- [x] Implement _actual_ healthchecks (steal them from osde2e) to determine cluster stability
