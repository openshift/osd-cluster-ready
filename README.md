# OSD Cluster Readiness Job

This job silences alerts while Day2 configuration is loaded onto a cluster at initial provisioning, allowing it to not page on-call SREs for normal operations within the cluster.  

The silence takes effect for 1 hour, which allows any clusters that are having issues to alert an SRE for them to actually look at.

## Deploying the Image

```GOOS=linux go build -o ./bin/main main.go && docker build . -t quay.io/kbater/openshift-cli && docker push quay.io/kbater/openshift-cli```

This builds the binary for linux, builds the docker file (which requires the binary to be built externally as of right now) and then pushes the updated image to quay.

## Deploying the Job

Deploy each of the jobs in alpha/numeric order in the deploy folder.

# TO DO

[ ] Skip the job if the cluster is older than 1 hour  
[ ] Implement _actual_ healthchecks (steal them from osde2e) to determine cluster stability  
[ ] Find if there's a better and more secure way to talk to the alertmanager API using oauth and serviceaccount tokens.  
