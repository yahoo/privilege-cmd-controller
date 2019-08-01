// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type remoteCmdExecutor struct {
	client             kubernetes.Interface
	restConfig         *rest.Config
	namespace          string
	requestSpec        *requestSpec
	container          string
	containerID        string
	privilegeContainer string
}

// executeNsenterCommand executes the specified command from user through using nsenter using privilege pod
func executeNsenterCommand(rce *remoteCmdExecutor, command []string) (string, error) {
	// retrieve PID
	pid, err := getPID(rce)
	if err != nil {
		return "", errors.New("unable to execute command: " + err.Error())
	}

	// Execute privileged command
	// nsenter with mount, uts, net and pid namespaces
	// example: nsenter --target 28400 --mount --ipc --uts --net --pid gcore 1
	commandToExecute := append([]string{"nsenter", "--target", pid, "--ipc", "--uts", "--net", "--pid"}, command...)
	glog.Infof("[%s] Command to execute on pod %s under namespace %s: %v", rce.requestSpec.reqID, rce.requestSpec.privPodName, rce.namespace, commandToExecute)

	return execCommandOnPod(rce, commandToExecute)
}

// getPID retrieves the PID of the target container
func getPID(rce *remoteCmdExecutor) (string, error) {
	// Construct command for retrieving PID
	// Example command for retrieving PID: docker inspect --format '{{ .State.Pid }}' 99ba788b9c7c99a86c3fc2dd400e2d9cb5312d8e5b4f4fb9500b18e1a406226f
	glog.Infof("[%s] Retrieving PID for target container %s with container ID %s", rce.requestSpec.reqID, rce.container, rce.containerID)
	command := []string{"docker", "inspect", "--format", "'{{ .State.Pid }}'", rce.containerID}
	glog.Infof("[%s] Command for retrieving PID for container %s with container ID %s: %v", rce.requestSpec.reqID, rce.container, rce.containerID, command)

	pid, err := execCommandOnPod(rce, command)
	if err != nil {
		return "", fmt.Errorf("unable to retrieve PID for container %s with container ID %s: %s", rce.container, rce.containerID, err)
	}

	// Fix issue with prefix after retrieving value from script
	pid = pid[1 : len(pid)-2]
	glog.Infof("[%s] Retrieved successfully PID for container %s: %s", rce.requestSpec.reqID, rce.container, pid)
	return pid, err
}
