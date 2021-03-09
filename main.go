package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/openshift/osde2e/pkg/common/cluster"
)

const (
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

	for {
		healthy, err := isClusterHealthy(cleanCheckRuns, cleanCheckInterval)
		if err != nil {
			log.Fatal(err)
		}
		if healthy {
			os.Exit(0)
		}

		log.Printf("Health checks failed. Sleeping %d seconds before rechecking...\n", failedCheckInterval)
		time.Sleep(time.Duration(failedCheckInterval) * time.Second)
	}

	// UNREACHED
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
