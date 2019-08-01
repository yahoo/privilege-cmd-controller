// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package k8sutil

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClient returns a Kubernetes client (clientset) from the kubeconfig path
// or from the in-cluster service account environment.
func GetClient(path string) (*rest.Config, *kubernetes.Clientset, error) {
	conf, err := getClientConfig(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get Kubernetes client config: %v", err)
	}
	client, err := kubernetes.NewForConfig(conf)
	return conf, client, err
}

// getClientConfig returns a Kubernetes client Config.
func getClientConfig(path string) (*rest.Config, error) {
	if path != "" {
		glog.Infof("Use Kubernetes client config from kubeconfig file %s", path)
		// build Config from a kubeconfig filepath
		return clientcmd.BuildConfigFromFlags("", path)
	}
	glog.Info("Use in-cluster Kubernetes client config")
	// uses pod's service account to get a Config
	return rest.InClusterConfig()
}
