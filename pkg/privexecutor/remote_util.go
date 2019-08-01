// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/golang/glog"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// execCommandOnPod executes a given command on the target pod
func execCommandOnPod(rce *remoteCmdExecutor, command []string) (string, error) {
	restclient := rce.client.CoreV1().RESTClient()

	req := restclient.Post().
		Namespace(rce.namespace).
		Resource("pods").
		Name(rce.requestSpec.privPodName).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: rce.privilegeContainer,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	// Set executor for the target pod
	executor, err := remotecommand.NewSPDYExecutor(rce.restConfig, http.MethodPost, req.URL())
	if err != nil {
		return "", fmt.Errorf("command %v failed to set SPDY executor: %s", command, err)
	}

	var stdout, stderr bytes.Buffer
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// Handle std error if any have been returned
	if stderr.Len() > 0 {
		glog.Errorf("[%s] Command %v returned std err: %v", rce.requestSpec.reqID, command, stderr.String())
	}

	// Handle errors from executing the remote command
	if err != nil {
		return "", fmt.Errorf("command %v failed: %s", command, err)
	}

	return stdout.String(), nil
}
