package rosa

import (
	"context"
	"fmt"
	"os/exec"
)

// operatorRoleError represents the custom error
type operatorRoleError struct {
	action string
	err    error
}

// Error returns the formatted error message when operatorRoleError is invoked
func (o *operatorRoleError) Error() string {
	return fmt.Sprintf("%s operator role failed: %v", o.action, o.err)
}

// deleteOIDCConfigProvider deletes the oidc config provider associated to the cluster
func (r *Provider) deleteOperatorRoles(ctx context.Context, clusterID, clusterPrefix, oidcConfigID string) error {
	commandArgs := []string{
		"delete", "operator-roles",
		"--mode", "auto",
		"--yes",
	}

	if oidcConfigID != "" {
		commandArgs = append(commandArgs, "--prefix", clusterPrefix)
	} else {
		commandArgs = append(commandArgs, "--cluster", clusterID)
	}

	r.log.Info("Deleting cluster operator roles", clusterIDLoggerKey, clusterID, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	_, stderr, err := r.RunCommand(ctx, exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
	if err != nil {
		return &operatorRoleError{action: "delete", err: fmt.Errorf("error: %v, stderr: %s", err, stderr.String())}
	}

	r.log.Info("Cluster operator roles deleted!", clusterIDLoggerKey, clusterID, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	return nil
}
