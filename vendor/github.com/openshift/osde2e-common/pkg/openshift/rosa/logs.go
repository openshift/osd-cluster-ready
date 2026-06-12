package rosa

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// clusterLog gets the clusters log for the provided type and writes it to a file
func (r *Provider) clusterLog(ctx context.Context, logType, clusterName, reportDir string) error {
	switch logType {
	case "install", "uninstall":
	default:
		return fmt.Errorf("cluster log type must be either install or uninstall, received: %s", logType)
	}

	commandArgs := []string{
		"logs", logType,
		"--cluster", clusterName,
	}

	r.log.Info("Get cluster log", clusterLogTypeLoggerKey, logType, clusterNameLoggerKey, clusterName, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	stdout, _, err := r.RunCommand(ctx, exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
	if err != nil {
		return fmt.Errorf("failed to get cluster %s log: %v", logType, err)
	}

	if err = os.WriteFile(fmt.Sprintf("%s/%s-%s.log", reportDir, clusterName, logType), []byte(fmt.Sprint(stdout)), os.FileMode(0o644)); err != nil {
		return fmt.Errorf("failed to cluster %s log to file: %v", logType, err)
	}

	r.log.Info("Cluster log retrieved!", clusterLogTypeLoggerKey, logType, clusterNameLoggerKey, clusterName, ocmEnvironmentLoggerKey, r.ocmEnvironment)

	return nil
}
