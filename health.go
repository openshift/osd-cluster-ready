package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/go-multierror"
	configv1 "github.com/openshift/api/config/v1"
	osconfig "github.com/openshift/client-go/config/clientset/versioned"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/osde2e/pkg/common/cluster"
	"github.com/openshift/osde2e/pkg/common/cluster/healthchecks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const apiServerName = "cluster"

// pollClusterHealth runs the same osde2e health checks as cluster.PollClusterHealth,
// except the certificate check reads APIServer namedCertificates instead of listing Secrets.
func pollClusterHealth(clusterID string, logger *log.Logger) (bool, []string, error) {
	if logger == nil {
		logger = log.Default()
	}

	logger.Print("Polling Cluster Health...\n")

	restConfig, providerType, err := cluster.ClusterConfig(clusterID)
	if err != nil {
		logger.Printf("Error getting cluster config: %v\n", err)
		return false, nil, nil
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logger.Printf("Error generating Kube Clientset: %v\n", err)
		return false, nil, nil
	}

	oscfg, err := osconfig.NewForConfig(restConfig)
	if err != nil {
		logger.Printf("Error generating OpenShift Clientset: %v\n", err)
		return false, nil, nil
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		logger.Printf("Error generating Dynamic Clientset: %v\n", err)
		return false, nil, nil
	}

	clusterHealthy := true
	var healthErr *multierror.Error
	var failures []string

	switch providerType {
	case "rosa":
		fallthrough
	case "ocm":
		if check, err := healthchecks.CheckCVOReadiness(oscfg.ConfigV1(), logger); !check || err != nil {
			healthErr = multierror.Append(healthErr, err)
			failures = append(failures, "cvo")
			clusterHealthy = false
		}

		if check, err := healthchecks.CheckNodeHealth(kubeClient.CoreV1(), logger); !check || err != nil {
			healthErr = multierror.Append(healthErr, err)
			failures = append(failures, "node")
			clusterHealthy = false
		}

		if check, err := healthchecks.CheckMachinesObjectState(dynamicClient, logger); !check || err != nil {
			healthErr = multierror.Append(healthErr, err)
			failures = append(failures, "machine")
			clusterHealthy = false
		}

		if check, err := healthchecks.CheckOperatorReadiness(oscfg.ConfigV1(), logger); !check || err != nil {
			healthErr = multierror.Append(healthErr, err)
			failures = append(failures, "operator")
			clusterHealthy = false
		}

		if check, err := checkCerts(oscfg.ConfigV1(), logger); !check || err != nil {
			healthErr = multierror.Append(healthErr, err)
			failures = append(failures, "cert")
			clusterHealthy = false
		}

		if check, err := healthchecks.CheckReplicaCountForDaemonSets(kubeClient.AppsV1(), logger); !check || err != nil {
			healthErr = multierror.Append(healthErr, err)
			failures = append(failures, "daemonset")
			clusterHealthy = false
		}

		if check, err := healthchecks.CheckReplicaCountForReplicaSets(kubeClient.AppsV1(), logger); !check || err != nil {
			healthErr = multierror.Append(healthErr, err)
			failures = append(failures, "replicaset")
			clusterHealthy = false
		}

	default:
		logger.Printf("No provisioner-specific logic for %q", providerType)
	}

	return clusterHealthy, failures, healthErr.ErrorOrNil()
}

// checkCerts reports whether Hive has applied a serving certificate to the cluster APIServer.
func checkCerts(configClient configv1client.ConfigV1Interface, logger *log.Logger) (bool, error) {
	if logger == nil {
		logger = log.Default()
	}

	apiserver, err := configClient.APIServers().Get(context.TODO(), apiServerName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("error reading APIServer %q: %w", apiServerName, err)
	}

	if !hasNamedServingCertificate(apiserver.Spec.ServingCerts.NamedCertificates) {
		logger.Printf("Certificate(s) not yet issued.")
		return false, nil
	}

	logger.Printf("Certificate(s) has been found.")
	return true, nil
}

func hasNamedServingCertificate(certs []configv1.APIServerNamedServingCert) bool {
	for _, cert := range certs {
		if cert.ServingCertificate.Name != "" {
			return true
		}
	}
	return false
}
