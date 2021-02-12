package silence

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

const createdBy = "OSD Cluster Readiness Job"

type SilenceRequest struct {
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

type getSilenceResponse []*SilenceRequest

type silenceResponse struct {
	ID string `json:"silenceID"`
}

func NewSilenceRequest() *SilenceRequest {
	return &SilenceRequest{}
}

// FindExisting looks for an existing, active silence that was created by us. If found,
// its ID is returned; otherwise the empty string is returned. The latter is not an
// error condition.
func (sr *SilenceRequest) FindExisting() (*SilenceRequest, error) {
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
			return sr, err
		}
		if len(silences) == 0 {
			log.Printf("No Silences Present")
			return sr, nil
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
			return sr, nil
		}

		log.Printf("No silences created by job found.")
		return sr, nil
	}

	return sr, fmt.Errorf("unable to get a list of existing silences")
}

func (sr *SilenceRequest) Build(expiryPeriod time.Duration) *SilenceRequest {
	// Create the Silence
	now := time.Now().UTC()
	end := now.Add(expiryPeriod)

	allMatcher := matcher{}
	allMatcher.Name = "severity"
	// Match all alerts
	allMatcher.Value = ".*"
	allMatcher.IsRegex = true

	sr.Matchers = []matcher{allMatcher}
	sr.StartsAt = now.Format(time.RFC3339)
	sr.EndsAt = end.Format(time.RFC3339)
	sr.CreatedBy = createdBy
	sr.Comment = "Created By the Cluster Readiness Job to silence any alerts during normal provisioning"

	return sr
}

// Create adds a new silence that expires in one hour.
func (sr *SilenceRequest) Create() (*SilenceRequest, error) {

	silenceJSON, err := json.Marshal(sr)
	if err != nil {
		return sr, fmt.Errorf("There was an error marshalling JSON: %v", silenceJSON)
	}

	return sr, nil
}

func (sr *SilenceRequest) Send(silenceJSON []byte) (*silenceResponse, error) {
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
			return nil, fmt.Errorf("There was an error Unmarshalling response: %v", e)
		}
		log.Printf("Silence Created with ID %s.", silenceResp.ID)
		return &silenceResp, nil
	}
}

// Remove deletes the silence with the given silenceID
func (sr *SilenceRequest) Remove() error {
	log.Printf("Removing Silence %s\n", sr.ID)
	for i := 0; i < 5; i++ {
		// Attempt up to 5 times to unsilence the cluster
		unsilenceCommand := exec.Command("oc", "exec", "-n", "openshift-monitoring", "alertmanager-main-0", "-c", "alertmanager", "--", "curl", fmt.Sprintf("localhost:9093/api/v2/silence/%s", sr.ID), "--silent", "-X", "DELETE")
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

// ExpiresIn returns bool if the remaining time on the AlertManager Silence is less than the expiryPeriod
func (sr *SilenceRequest) WillExpireBy(expiryPeriod time.Duration) (bool, error) {
	// Parse start and end times of Alertmanager Silence
	start, err := time.Parse(time.RFC3339, sr.StartsAt)
	if err != nil {
		return false, err
	}
	end, err := time.Parse(time.RFC3339, sr.EndsAt)
	if err != nil {
		return false, err
	}

	// Find the remaining time left on the silence
	remaining := end.Sub(start)

	// Return bool if the remaining time is less than the expiryPeriod
	return remaining < expiryPeriod, nil
}
