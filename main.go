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

	"github.com/openshift/osd-cluster-ready/silence"
)

const (
	// Maximum cluster age, in minutes, before we'll start ignoring it.
	// This is in case the Job gets deployed on an already-initialized but unhealthy cluster:
	// we don't want to silence alerts in that case.
	maxClusterAgeKey = "MAX_CLUSTER_AGE_MINUTES"
	// By default, ignore clusters older than two hours
	maxClusterAgeDefault = 2 * 60

	// The number of consecutive health checks that must succeed before we declare the cluster
	// truly healthy.
	cleanCheckRunsKey     = "CLEAN_CHECK_RUNS"
	cleanCheckRunsDefault = 20

	// The number of seconds between successful health checks.
	cleanCheckIntervalKey     = "CLEAN_CHECK_INTERVAL_SECONDS"
	cleanCheckIntervalDefault = 30

	// The number of seconds to sleep after a failed health check.
	failedCheckIntervalKey     = "FAILED_CHECK_INTERVAL_SECONDS"
	failedCheckIntervalDefault = 60
)

func main() {
	// New Alert Manager Silence Request
	// TODO: Handle multiple silences being active
	silenceReq := silence.New()

	clusterBirth, err := getClusterCreationTime()
	if err != nil {
		log.Fatal(err)
	}

	maxClusterAge, err := getEnvInt(maxClusterAgeKey, maxClusterAgeDefault)
	if err != nil {
		log.Fatal(err)
	}

	cleanCheckRuns, err := getEnvInt(cleanCheckRunsKey, cleanCheckRunsDefault)
	if err != nil {
		log.Fatal(err)
	}

	cleanCheckInterval, err := getEnvInt(cleanCheckIntervalKey, cleanCheckIntervalDefault)
	if err != nil {
		log.Fatal(err)
	}

	failedCheckInterval, err := getEnvInt(failedCheckIntervalKey, failedCheckIntervalDefault)
	if err != nil {
		log.Fatal(err)
	}

	// maxHealthCheckDuration is a heuristic for how long the full health check
	// loop should take. This is used to push out the expiry of active silences to make
	// sure they don't expire while we're in that loop; so err on the high side:
	// - One run of health checks is pretty fast -- anecdotally sub-second -- but give it
	//   ten seconds.
	// - Add failedCheckInterval since we do that sleep *after* a failed health check.
	// - Add another two minutes of fudge factor.
	maxHealthCheckDuration := time.Duration(cleanCheckRuns * (cleanCheckInterval + 10) + failedCheckInterval + 120) * time.Second
	log.Printf("Using a silence expiry window of %s.", maxHealthCheckDuration)

	for {

		// TODO: If silenceReq has an ID here, no need to look for another active silence
		silenceReq, err := silenceReq.FindExisting()
		if err != nil {
			log.Fatal(err)
		}

		if clusterTooOld(clusterBirth, maxClusterAge) {
			log.Printf("Cluster is older than %d minutes. Exiting Cleanly.", maxClusterAge)
			if silenceReq.Active() {
				// TODO: There might be more than one silence to remove
				err = silenceReq.Remove()
				if err != nil {
					log.Fatal(err)
				}
			}
			os.Exit(0)
		}

		if !silenceReq.Active() {
			// If there are no existing active silences, create one.
			// We only get here in two scenarios:
			// - This is the first iteration of the loop. The vast majority of the time
			//   this is the first ever run of the job, and it's happening shortly after
			//   the birth of the cluster.
			// - The previous iteration took longer than maxHealthCheckDuration
			//   and the previously-created silence expired.
			_, err = silenceReq.Build(maxHealthCheckDuration).Send()
			if err != nil {
				log.Fatal(err)
			}

		} else {
			// If there is an existing silence, ensure it's not going to expire before
			// our healthchecks are complete.
			willExpire, err := silenceReq.WillExpireBy(maxHealthCheckDuration)
			if err != nil {
				log.Fatal(err)
			}
			if willExpire {
				log.Printf("Silence expires in less than %s. Extending.", maxHealthCheckDuration)
				// Creating a new silence automatically expires the existing one(s).
				_, err = silenceReq.Build(maxHealthCheckDuration).Send()
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		healthy, err := isClusterHealthy(cleanCheckRuns, cleanCheckInterval)
		if err != nil {
			log.Fatal(err)
		}

		if healthy {
			// Use this to log how much time was left in the active silence. This can
			// help us fine-tune maxHealthCheckDuration in the future.
			// TODO: If this is negative, it has already expired and we don't have to
			// remove it. But WillExpireBy would have to tell us that somehow.
			_, err = silenceReq.WillExpireBy(maxHealthCheckDuration)
			// TODO: There might be more than one silence to remove
			err = silenceReq.Remove()
			if err != nil {
				log.Fatal(err)
			}
			os.Exit(0)
		}

		log.Printf("Health checks failed. Sleeping %d seconds before rechecking...\n", failedCheckInterval)
		time.Sleep(time.Duration(failedCheckInterval) * time.Second)
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

// getEnvInt returns the integer value of the environment variable with the specified `key`.
// If the env var is unspecified/empty, the `def` value is returned.
// The error is non-nil if the env var is nonempty but cannot be parsed as an int.
func getEnvInt(key string, def int) (int, error) {
	var intVal int
	var err error

	strVal := os.Getenv(key)

	if strVal == "" {
		// Env var unset; use the default
		return def, nil
	}

	if intVal, err = strconv.Atoi(strVal); err != nil {
		return 0, fmt.Errorf("Invalid value for env var: %s=%s (expected int): %v", key, strVal, err)
	}

	return intVal, nil
}

func clusterTooOld(clusterBirth time.Time, maxAgeMinutes int) bool {
	maxAge := time.Now().Add(time.Duration(-maxAgeMinutes) * time.Minute)
	return clusterBirth.Before(maxAge)
}

// doHealthCheck performs one instance of the health check.
// Logs what happens.
// Returns (true, err) if all health checks succeeded.
// Returns (false, err) if any health check failed.
// Iff an error occurs, err is non-nil.
func doHealthCheck() (bool, error) {
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

// isClusterHealthy runs health checks multiple times, succeeding only if checks pass the requisite
// number of consecutive times. We return failure immediately if any check fails, or if an error
// occurs.
func isClusterHealthy(cleanCheckRuns, cleanCheckInterval int) (bool, error) {
	for i := 1; ; i++ {
		log.Printf("======== Health Checks: %d of %d ========\n", i, cleanCheckRuns)
		status, err := doHealthCheck()
		if err != nil {
			return false, err
		}
		if status && i >= cleanCheckRuns {
			return true, nil
		}
		if !status {
			return false, nil
		}
		log.Printf("Sleeping %d seconds...\n", cleanCheckInterval)
		time.Sleep(time.Duration(cleanCheckInterval) * time.Second)
	}
}
