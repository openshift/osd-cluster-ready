# OSD Cluster Readiness Job

This job silences alerts while Day2 configuration is loaded onto a cluster at initial provisioning, allowing it to not page on-call SREs for normal operations within the cluster.  

The silence initially takes effect for 1 hour.

We then poll cluster health using [osde2e health checks](https://github.com/openshift/osde2e/blob/041355675304a7aa371b7fbeea313001036feb75/pkg/common/cluster/clusterutil.go#L211) once a minute, forever, until they all report healthy.

If the silence expires while health checks are failing, we reinstate it.
(This means it is theoretically possible for alerts to fire for up to one minute if the silence expires right after a health check fails. FIXME?)

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

## CAVEAT!

In real life, this Job is intended to spin up early in a cluster's life (while it is still being "created") and silence alerts until the dust settles.
If it were to run in an already-settled but *unhealthy* cluster, **it would silence alerts until the cluster becomes healthy**. So don't do that.

# TO DO

[x] Look for existing active silences before creating a new one
[x] Implement _actual_ healthchecks (steal them from osde2e) to determine cluster stability
[ ] Find if there's a better and more secure way to talk to the alertmanager API using oauth and serviceaccount tokens.  
