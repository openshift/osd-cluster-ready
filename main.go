package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	osconfig "github.com/openshift/client-go/config/clientset/versioned"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

	silenceID, err := checkForExistingSilence()
	if err != nil {
		log.Fatal(err)
	}
	if silenceID == "" {
		silenceID, err = createSilence()
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Beginning Healthchecks.")
	time.Sleep(30 * time.Minute) // TODO: Remove this and do loop thru actual healthchecks
	log.Println("Healthchecks Succeeded.  Removing Silence.")

	err = removeSilence(silenceID)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Silence Successfully Removed.")
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
		return nil
	}
	return fmt.Errorf("there was an error unsilencing the cluster")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func getClientSet() (*osconfig.Clientset, error) {
	var kubeconfig *string
	// Attempt to use the inClusterConfig first and then if that doesn't work fall back to local
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Println("No in-cluster config present.  Attempting to use local config.")
		if home := homeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			log.Fatal("Cannot build config")
		}
	}
	return osconfig.NewForConfig(config)
}
