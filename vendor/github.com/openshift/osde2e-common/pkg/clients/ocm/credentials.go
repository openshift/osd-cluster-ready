package ocm

import (
	"context"
	"fmt"
	"os"
)

// getKubeconfig returns the clusters kubeconfig content
func (c *Client) getKubeconfig(ctx context.Context, clusterID string) (string, error) {
	response, err := c.ClustersMgmt().V1().Clusters().Cluster(clusterID).Credentials().Get().SendContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get credentials for cluster id %q: %v", clusterID, err)
	}
	return response.Body().Kubeconfig(), nil
}

// GetKubeconfig returns the clusters kubeconfig file
func (c *Client) KubeconfigFile(ctx context.Context, clusterID, directory string) (string, error) {
	filename := fmt.Sprintf("%s/%s-kubeconfig", directory, clusterID)

	kubeconfig, err := c.getKubeconfig(ctx, clusterID)
	if err != nil {
		return filename, err
	}

	err = os.WriteFile(filename, []byte(kubeconfig), 0o600)
	if err != nil {
		return filename, fmt.Errorf("failed to write kubeconfig file: %v", err)
	}

	return filename, nil
}

// Kubeconfig returns the clusters kubeconfig content
func (c *Client) Kubeconfig(ctx context.Context, clusterID string) (string, error) {
	return c.getKubeconfig(ctx, clusterID)
}
