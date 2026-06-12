package rosa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os/exec"

	"github.com/openshift/osde2e-common/internal/cmd"
)

// regionError represents the custom error
type regionError struct {
	action string
	err    error
}

// Error returns the formatted error message when regionError is invoked
func (r *regionError) Error() string {
	return fmt.Sprintf("region %s failed: %v", r.action, r.err)
}

// region represents a rosa aws region object
type region struct {
	ID                 string        `json:"id"`
	Href               string        `json:"href"`
	CCSOnly            bool          `json:"ccs_only"`
	CloudProvider      cloudProvider `json:"cloud_provider"`
	DisplayName        string        `json:"display_name"`
	Enabled            bool          `json:"enabled"`
	SupportsHyperShift bool          `json:"supports_hypershift"`
	SupportsMultiAZ    bool          `json:"supports_multi_az"`
}

// cloudProvider represents a rosa aws region cloud provider object
type cloudProvider struct {
	ID   string `json:"id"`
	Href string `json:"href"`
}

// regionCheck verifies the region provided supports either hosted control plane clusters
// or multi az clusters based on the cluster creation options
func (r *Provider) regionCheck(ctx context.Context, regionName string, hostedCP, multiAZ bool) error {
	const action = "check"
	regionFound := false

	regions, err := r.regions(ctx, hostedCP, multiAZ)
	if err != nil {
		return &regionError{action: action, err: err}
	}

	r.log.Info("Performing ROSA AWS region check", "region", regionName, "hosted_cp", hostedCP, "multi_az", multiAZ)

	for _, region := range regions {
		if region.ID != regionName {
			continue
		}

		regionFound = true

		if !region.Enabled {
			return &regionError{action: action, err: fmt.Errorf("region %q is not enabled", regionName)}
		}

		break
	}

	if !regionFound {
		return &regionError{action: action, err: fmt.Errorf("region %q is not enabled/valid for the aws account in use and "+
			"supports: hostedCP=%t, multiAZ=%t", regionName, hostedCP, multiAZ)}
	}

	r.log.Info("ROSA AWS region check passed", "region", regionName, "hosted_cp", hostedCP, "multi_az", multiAZ)

	return nil
}

// selectRandomRegion selects a random enabled aws region to use
func (r *Provider) selectRandomRegion(ctx context.Context) (string, error) {
	r.log.Info("Selecting random aws region")

	regions, err := r.regions(ctx, false, false)
	if err != nil {
		return "", err
	}

	var enabledRegions []string

	for _, region := range regions {
		if region.Enabled {
			enabledRegions = append(enabledRegions, region.ID)
		}
	}

	rand.Shuffle(len(enabledRegions), func(i, j int) {
		enabledRegions[i], enabledRegions[j] = enabledRegions[j], enabledRegions[i]
	})

	if len(enabledRegions) == 0 {
		return "", &regionError{action: "select random region", err: errors.New("no regions found")}
	}

	selectedRegion := enabledRegions[0]

	r.log.Info("Random aws region selected!", awsRegionLoggerKey, selectedRegion)

	return selectedRegion, nil
}

// regions returns a list of available aws regions for the rh/aws account used
func (r *Provider) regions(ctx context.Context, hostedCP, multiAZ bool) ([]*region, error) {
	const action = "list"

	commandArgs := []string{
		"list", "regions",
		"--output", "json",
	}

	if hostedCP {
		commandArgs = append(commandArgs, "--hosted-cp")
	}

	if multiAZ {
		commandArgs = append(commandArgs, "--multi-az")
	}

	r.log.Info("Performing ROSA list regions", "hosted_cp", hostedCP, "multi_az", multiAZ)

	stdout, stderr, err := r.RunCommand(ctx, exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
	if err != nil {
		return nil, &regionError{action: action, err: fmt.Errorf("error: %v, stderr: %s", err, stderr.String())}
	}

	availableRegions, err := cmd.ConvertOutputToListOfMaps(stdout)
	if err != nil {
		return nil, &regionError{action: action, err: fmt.Errorf("failed to convert output to list of maps: %v", err)}
	}

	var regions []*region

	availableRegionsBytes, err := json.Marshal(availableRegions)
	if err != nil {
		return nil, &regionError{err: fmt.Errorf("failed to marshal region data: %v", err)}
	}

	err = json.Unmarshal(availableRegionsBytes, &regions)
	if err != nil {
		return nil, &regionError{action: action, err: fmt.Errorf("failed to unmarshal region data: %v", err)}
	}

	return regions, nil
}
