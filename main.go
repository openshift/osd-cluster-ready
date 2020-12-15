package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/iamkirkbater/osd-readiness-spike/healthchecks"
	osconfig "github.com/openshift/client-go/config/clientset/versioned"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	clientset, err := getClientSet()
	if err != nil {
		panic(err.Error())
	}

	// Create the Silence

	// Run the healthchecks
	if check, err := healthchecks.CheckCVOReadiness(clientset.ConfigV1()); !check || err != nil {
		log.Printf("Failed")
	}
	time.sleep(10 * time.Minute) // TODO: Remove this and do loop thru actual healthchecks
	log.Println("Success")
	// Once healthchecks return successfully, remove the silence

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
