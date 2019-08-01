// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package main

import (
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
	"github.com/yahoo/privilege-cmd-controller/pkg/k8sutil"
	"github.com/yahoo/privilege-cmd-controller/pkg/privexecutor"
)

// main initiates the Privilege Command Controller across all namespaces
func main() {
	// If specified, client is created from kubeconfig file else, it is created from
	// pods service account which injected in to pod. Using local config can make
	// development and testing easier.
	kubeconfig := flag.StringP("kubeconfig", "c", "", "Path to a kubeconfig file")
	privilegePodImage := flag.StringP("privilegePodImage", "i", "", "Image for the privileged pod to be created on target node. It contains all related privileged command utilities.")
	_ = flag.StringP("namespace", "n", "kube-pcc", "Namespace for privileged pod to be scheduled")
	_ = flag.StringP("serviceaccount", "s", "kube-priv-pod", "Service account for privileged pod to be scheduled")
	_ = flag.IntP("privPodTimeout", "t", 300, "Timeout for checking running status of the privilege pod in seconds")
	_ = flag.Set("logtostderr", "true")
	flag.Parse()

	if *privilegePodImage == "" {
		glog.Fatal("No privilege pod image passed in through command line. Specify privilege pod image with 'privilegePodImage' flag")
	}

	// Get kubernetes client and rest config
	restConfig, client, err := k8sutil.GetClient(*kubeconfig)
	if err != nil {
		glog.Errorf("Failed to create kubernetes in-cluster client: %v", err)
		return
	}

	// Initiate Privilege Command Controller
	stop := make(chan struct{})
	privilegeCmdController := privexecutor.NewPrivilegeCmdController(client, restConfig)
	defer close(stop)
	privilegeCmdController.Controller.Run(stop)
}
