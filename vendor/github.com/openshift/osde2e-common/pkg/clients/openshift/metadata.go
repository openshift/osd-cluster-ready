package openshift

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

const (
	metadataConfigMap = "osd-cluster-metadata"
	configNamespace   = "openshift-config"
)

// getOsdClusterMetadata returns osd-cluster-metadata configmap data array from openshift-config namespace
// this contains metadata about the cluster
func (c Client) getOsdClusterMetadata(ctx context.Context) (map[string]string, error) {
	var cm corev1.ConfigMap
	if err := c.Get(ctx, metadataConfigMap, configNamespace, &cm); err != nil {
		return nil, err
	}
	return cm.Data, nil
}

func (c Client) IsSTS(ctx context.Context) (bool, error) {
	cmData, err := c.getOsdClusterMetadata(ctx)
	if err != nil {
		return false, err
	}
	return cmData["api_openshift_com_sts"] == "true", nil
}

func (c Client) IsCCS(ctx context.Context) (bool, error) {
	cmData, err := c.getOsdClusterMetadata(ctx)
	if err != nil {
		return false, err
	}
	return cmData["api_openshift_com_ccs"] == "true", nil
}

func (c Client) GetProvider(ctx context.Context) (string, error) {
	cmData, err := c.getOsdClusterMetadata(ctx)
	if err != nil {
		return "", err
	}
	return cmData["hive_openshift_io_cluster-platform"], nil
}

func (c Client) GetRegion(ctx context.Context) (string, error) {
	cmData, err := c.getOsdClusterMetadata(ctx)
	if err != nil {
		return "", err
	}
	return cmData["hive_openshift_io_cluster-region"], nil
}
