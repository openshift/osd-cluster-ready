package rosa

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/openshift/osde2e-common/internal/cmd"

	clustersmgmtv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

// oidcConfigError represents the custom error
type oidcConfigError struct {
	action string
	err    error
}

// Error returns the formatted error message when oidcConfigError is invoked
func (o *oidcConfigError) Error() string {
	return fmt.Sprintf("%s oidc config failed: %v", o.action, o.err)
}

// createOIDCConfig creates an oidc config if one does not already exist
func (r *Provider) CreateOIDCConfig(ctx context.Context, prefix, installerRoleArn string) (string, error) {
	const action = "create"

	if prefix == "" || installerRoleArn == "" {
		return "", &oidcConfigError{action: action, err: errors.New("some parameters are undefined")}
	}

	oidcConfig, err := r.oidcConfigLookup(ctx, prefix)
	if oidcConfig != nil {
		r.log.Info("OIDC config id already exist", prefixLoggerKey, prefix, oidcConfigIDLoggerKey, oidcConfig.ID(),
			ocmEnvironmentLoggerKey, r.ocmEnvironment)
		return oidcConfig.ID(), nil
	} else if err != nil {
		return "", &oidcConfigError{action: action, err: err}
	}

	commandArgs := []string{
		"create", "oidc-config",
		"--output", "json",
		"--mode", "auto",
		"--yes",
	}

	// The OIDC needs to be `--managed` for FedRamp Which does not support these flags: --prefix, --installer-role-arn
	if !r.fedRamp {
		commandArgs = append(commandArgs, "--managed=false")
		commandArgs = append(commandArgs, "--prefix", prefix)
		commandArgs = append(commandArgs, "--installer-role-arn", installerRoleArn)
	}

	r.log.Info("Creating OIDC config", prefixLoggerKey, prefix, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	stdout, stderr, err := r.RunCommand(ctx, exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
	if err != nil {
		return "", &oidcConfigError{action: action, err: fmt.Errorf("error: %v, stderr: %s", err, stderr.String())}
	}

	output, err := cmd.ConvertOutputToMap(stdout)
	if err != nil {
		return "", fmt.Errorf("failed to convert output to map: %v", err)
	}

	r.log.Info("OIDC config created!", prefixLoggerKey, prefix, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	return fmt.Sprint(output["id"]), nil
}

// deleteOIDCConfig deletes the oidc config using the id
func (r *Provider) DeleteOIDCConfig(ctx context.Context, oidcConfigID string) error {
	commandArgs := []string{
		"delete", "oidc-config",
		"--mode", "auto",
		"--oidc-config-id", oidcConfigID,
		"--yes",
	}

	r.log.Info("Deleting OIDC config", oidcConfigIDLoggerKey, oidcConfigID, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	_, _, err := r.RunCommand(ctx, exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
	if err != nil {
		return &oidcConfigError{action: "delete", err: err}
	}

	r.log.Info("OIDC config deleted!", oidcConfigIDLoggerKey, oidcConfigID, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	return nil
}

// oidcConfigLookup checks if an oidc config already exists using the provided prefix
func (r *Provider) oidcConfigLookup(ctx context.Context, prefix string) (*clustersmgmtv1.OidcConfig, error) {
	response, err := r.ClustersMgmt().V1().OidcConfigs().List().SendContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve oidc configs from ocm: %v", err)
	}

	for _, oidcConfig := range response.Items().Slice() {
		if strings.Contains(oidcConfig.SecretArn(), prefix) {
			return oidcConfig, nil
		}
	}

	return nil, nil
}

// deleteOIDCConfigProvider deletes the oidc config provider associated to the cluster
func (r *Provider) deleteOIDCConfigProvider(ctx context.Context, clusterID, oidcConfigID string) error {
	commandArgs := []string{
		"delete", "oidc-provider",
		"--mode", "auto",
		"--yes",
	}

	if oidcConfigID != "" {
		commandArgs = append(commandArgs, "--oidc-config-id", oidcConfigID)
	} else {
		commandArgs = append(commandArgs, "--cluster", clusterID)
	}

	r.log.Info("Deleting cluster oidc config provider", clusterIDLoggerKey, clusterID, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	_, stderr, err := r.RunCommand(ctx, exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
	if err != nil {
		return &oidcConfigError{action: "delete", err: fmt.Errorf("error: %v, stderr: %s", err, stderr.String())}
	}

	r.log.Info("Cluster oidc config provider deleted!", clusterIDLoggerKey, clusterID, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	return nil
}
