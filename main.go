package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/osde2e/pkg/common/cluster"
)

const createdBy = "OSD Cluster Readiness Job"

type silenceRequest struct {
	ID        string       `json:"id"`
	Status    silenceState `json:"status"`
	Matchers  []matcher    `json:"matchers"`
	StartsAt  string       `json:"startsAt"`
	EndsAt    string       `json:"endsAt"`
	CreatedBy string       `json:"createdBy"`
	Comment   string       `json:"comment"`
}

type silenceState struct {
	State string `json:"state"`
}

type matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
}

type getSilenceResponse []*silenceRequest

type silenceResponse struct {
	ID string `json:"silenceID"`
}

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

		silenceID, err := checkForExistingSilence()
		if err != nil {
			log.Fatal(err)
		}

		if healthy {
			if silenceID != "" {
				err = removeSilence(silenceID)
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
			silenceID, err = createSilence()
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

func checkForExistingSilence() (string, error) {
	for i := 1; i <= 300; i++ { // try once a second or so for 5-ish minutes
		cmdstr := "oc exec -n openshift-monitoring alertmanager-main-0 -c alertmanager -- curl --silent localhost:9093/api/v2/silences -X GET"
		silenceGetCmd := exec.Command("bash", "-c", cmdstr)
		silenceGetCmd.Stderr = os.Stderr
		resp, err := silenceGetCmd.Output()
		if err != nil {
			log.Printf("Attempt %d to query for existing silences failed. %v", i, err)
			time.Sleep(1 * time.Second)
			continue
		}
		var silences getSilenceResponse
		err = json.Unmarshal(resp, &silences)
		if err != nil {
			log.Printf("There was an error unmarshalling get silence response")
			return "", err
		}
		if len(silences) == 0 {
			log.Printf("No Silences Present")
			return "", nil
		}

		for _, silence := range silences {
			if silence.CreatedBy != createdBy {
				continue
			}
			if silence.Status.State != "active" {
				log.Printf("Silence is not active.")
				continue
			}
			log.Printf("Found silence created by job: %s", silence.ID)
			return silence.ID, nil
		}

		log.Printf("No silences created by job found.")
		return "", nil
	}

	return "", fmt.Errorf("unable to get a list of existing silences")
}

func createSilence() (string, error) {
	// Create the Silence
	now := time.Now().UTC()
	end := now.Add(1 * time.Hour)

	allMatcher := matcher{}
	allMatcher.Name = "severity"
	allMatcher.Value = "info|warning|critical"
	allMatcher.IsRegex = true

	silenceBody := silenceRequest{}
	silenceBody.Matchers = []matcher{allMatcher}
	silenceBody.StartsAt = now.Format(time.RFC3339)
	silenceBody.EndsAt = end.Format(time.RFC3339)
	silenceBody.CreatedBy = createdBy
	silenceBody.Comment = "Created By the Cluster Readiness Job to silence any alerts during normal provisioning"

	silenceJSON, err := json.Marshal(silenceBody)
	if err != nil {
		return "", fmt.Errorf("There was an error marshalling JSON: %v", silenceJSON)
	}

	for {
		// Attempt to run once every 30 seconds until this succeeds
		// to account for if the alertmanager is not ready before
		// we start trying to silence it.
		silenceCmd := exec.Command("oc", "exec", "-n", "openshift-monitoring", "alertmanager-main-0", "-c", "alertmanager", "--", "curl", "localhost:9093/api/v2/silences", "--silent", "-X", "POST", "-H", "Content-Type: application/json", "--data", string(silenceJSON))
		silenceCmd.Stderr = os.Stderr
		resp, err := silenceCmd.Output()
		if err != nil {
			log.Printf("Silence Failed. %v", err)
			time.Sleep(30 * time.Second)
			continue
		}
		var silenceResp silenceResponse
		e := json.Unmarshal(resp, &silenceResp)
		if e != nil {
			return "", fmt.Errorf("There was an error Unmarshalling response: %v", e)
		}
		log.Printf("Silence Created with ID %s.", silenceResp.ID)
		return silenceResp.ID, nil
	}
}

func removeSilence(silenceID string) error {
	log.Printf("Removing Silence %s\n", silenceID)
	for i := 0; i < 5; i++ {
		// Attempt up to 5 times to unsilence the cluster
		unsilenceCommand := exec.Command("oc", "exec", "-n", "openshift-monitoring", "alertmanager-main-0", "-c", "alertmanager", "--", "curl", fmt.Sprintf("localhost:9093/api/v2/silence/%s", silenceID), "--silent", "-X", "DELETE")
		unsilenceCommand.Stderr = os.Stderr
		err := unsilenceCommand.Run()
		if err != nil {
			log.Printf("Attempt %d to unsilence failed. %v", i, err)
			time.Sleep(1 * time.Second)
			continue
		}
		log.Println("Silence Successfully Removed.")
		return nil
	}
	return fmt.Errorf("there was an error unsilencing the cluster")
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
