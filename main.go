package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/osde2e/pkg/common/cluster"

	"github.com/iamkirkbater/osd-cluster-ready-job/silence"
)

func main() {
	clusterInit, err := getClusterCreationTime()
	if err != nil {
		log.Fatal(err)
	}
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	if clusterInit.Before(oneHourAgo) {
		log.Printf("Cluster created more than 1 hour ago.  Exiting Cleanly.")
		os.Exit(0)
	}

	for {
		healthy, err := doHealthCheck()
		if err != nil {
			log.Fatal(err)
		}

		silenceID, err := silence.FindExisting()
		if err != nil {
			log.Fatal(err)
		}

		if healthy {
			if silenceID != "" {
				err = silence.Remove(silenceID)
				if err != nil {
					log.Fatal(err)
				}
			} else {
				log.Println("Health checks passed and cluster was not silenced. Nothing to do here.")
			}
			os.Exit(0)
		}

		// If we got here, our cluster is unhealthy. Make sure our silence is active.
		// We do this every time because the silence is set to expire automatically in an hour.
		if silenceID == "" {
			silenceID, err = silence.Create()
			if err != nil {
				log.Fatal(err)
			}
		}

		log.Println("Health checks failed. Sleeping...")
		time.Sleep(time.Minute)
	}

	// UNREACHED
}

func getClusterCreationTime() (time.Time, error) {
	for i := 1; i <= 300; i++ { // try once a second or so for 5 minutes-ish
		ex := "oc exec -n openshift-monitoring prometheus-k8s-0 -c prometheus -- curl localhost:9090/api/v1/query --silent --data-urlencode 'query=cluster_version' | jq -r '.data.result[] | select(.metric.type==\"initial\") | .value[1]'"
		promCmd := exec.Command("bash", "-c", ex)
		promCmd.Stderr = os.Stderr
		resp, err := promCmd.Output()
		if err != nil {
			log.Printf("Attempt %d to query for cluster age failed. %v", i, err)
			time.Sleep(1 * time.Second)
			continue
		}
		respTrimmed := strings.TrimSuffix(string(resp), "\n")
		initTime, err := strconv.ParseInt(respTrimmed, 10, 64)
		if err != nil {
			log.Printf("Error casting Epoch time to int. %s\nErr: %v", resp, err)
			time.Sleep(1 * time.Second)
			continue
		}
		clusterCreation := time.Unix(initTime, 0)
		log.Printf("Cluster Created %v", clusterCreation.UTC())
		return clusterCreation, nil
	}
	return time.Unix(0, 0), fmt.Errorf("there was an error getting cluster creation time")
}

// doHealthCheck performs one instance of the health check.
// Logs what happens.
// Returns (true, err) if all health checks succeeded.
// Returns (false, err) if any health check failed.
// Iff an error occurs, err is non-nil.
func doHealthCheck() (bool, error) {
	log.Println("======== Health Checks ========")
	status, failures, err := cluster.PollClusterHealth("", nil)
	if err != nil {
		log.Printf("Error(s) running health checks: %v\n", err)
	}
	if len(failures) != 0 {
		log.Printf("Healthcheck encountered the following failures: %v\n", failures)
	}
	if status {
		log.Printf("Health checks succeeded.")
	} else {
		log.Printf("Health check(s) failed.")
	}
	return status, err
}
