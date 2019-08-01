// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/yahoo/privilege-cmd-controller/pkg/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type requestSpec struct {
	privPodName string
	reqID       string
}

// Process handles changes in annotations and makes actions corresponding to the current status
func Process(pc *privilegeCmdController, oldPodResource *v1.Pod, newPodResource *v1.Pod, requestSpec *requestSpec) error {
	switch {
	// privileged-command-status annotation is active
	case newPodResource.Annotations[constants.AnnotationExecuteStatus] == constants.StatusActive:
		err := handleActiveStatus(pc, oldPodResource, newPodResource, requestSpec)
		if err != nil {
			return fmt.Errorf("unable to act upon annotation %s change to %s: %s", constants.AnnotationExecuteStatus, constants.StatusActive, err)
		}
		return nil
	// privileged-command-status annotation is done
	case newPodResource.Annotations[constants.AnnotationExecuteStatus] == constants.StatusDone:
		err := handleDoneStatus(pc, oldPodResource, newPodResource, requestSpec)
		if err != nil {
			return fmt.Errorf("unable to act upon annotation %s change to %s: %s", constants.AnnotationExecuteStatus, constants.StatusDone, err)
		}
		return nil
	}
	return nil
}

// handleActiveStatus takes necessary actions when privileged-command-status annotation switches to "active"
func handleActiveStatus(pc *privilegeCmdController, oldPodResource *v1.Pod, newPodResource *v1.Pod, requestSpec *requestSpec) error {
	// Retry action if it failed previously.
	if oldPodResource != nil && oldPodResource.Annotations != nil &&
		oldPodResource.Annotations[constants.AnnotationExecuteStatus] == constants.StatusActive {
		glog.Warningf("[%s] Retrying executing script now request as this update is due to previous error on pod %s", requestSpec.reqID, newPodResource.Name)
	}

	// Update privileged-command-status annotation to in-progress
	err := updatePrivilegedCommandExecutorAnnotation(pc.client, newPodResource.Namespace, newPodResource, constants.StatusInProgress, requestSpec)
	if err != nil {
		return fmt.Errorf("failed to update %s annotation to %s: %s", constants.AnnotationExecuteStatus, constants.StatusInProgress, err)
	}

	// Detect container ID of target container
	containerID := ""
	for _, p := range newPodResource.Status.ContainerStatuses {
		if p.Name == newPodResource.Annotations[constants.AnnotationExecuteContainer] {
			// container ID is in the form docker://<container_id>
			// so we trim the prefix docker://
			containerID = p.ContainerID[9:]
			glog.Infof("[%s] Container ID for container %s on pod %s under namespace %s: %s", requestSpec.reqID, newPodResource.Annotations[constants.AnnotationExecuteContainer], newPodResource.Name, newPodResource.Namespace, containerID)
		}
	}
	if containerID == "" {
		return fmt.Errorf("no matching container ID for container %s on pod %s under namespace %s", newPodResource.Annotations[constants.AnnotationExecuteContainer], newPodResource.Name, newPodResource.Namespace)
	}

	// Detect node name of the target pod
	nodeName, err := getNodeName(newPodResource)
	if err != nil {
		return errors.New("failed to detect target node: %s" + err.Error())
	}
	glog.Infof("[%s] Target node for container %s in pod %s is %s", requestSpec.reqID, newPodResource.Annotations[constants.AnnotationExecuteContainer], newPodResource.Name, nodeName)

	// Create privileged pod
	err = createPrivilegedPod(pc.client, CmdArgs.Namespace, nodeName, requestSpec)
	if err != nil {
		return err
	}

	// Set up remoteCmdExecutor object to execute remote command in the privileged pod
	rce := remoteCmdExecutor{
		client:             pc.client,
		restConfig:         pc.restConfig,
		namespace:          CmdArgs.Namespace,
		requestSpec:        requestSpec,
		container:          newPodResource.Annotations[constants.AnnotationExecuteContainer],
		containerID:        containerID,
		privilegeContainer: constants.PrivilegeContainer,
	}

	// Execute specified command supplied by annotations
	action := newPodResource.Annotations[constants.AnnotationExecuteAction]
	actionToExec := strings.Fields(action)
	output, err := executeNsenterCommand(&rce, actionToExec)
	if err != nil {
		return errors.New("failed to execute command: " + err.Error())
	}
	glog.Infof("[%s] \n%v", requestSpec.reqID, output)

	// Update privileged-command-status annotation to done
	err = updatePrivilegedCommandExecutorAnnotation(pc.client, newPodResource.Namespace, newPodResource, constants.StatusDone, requestSpec)
	if err != nil {
		return fmt.Errorf("failed to update %s annotation to %s: %s", constants.AnnotationExecuteStatus, constants.StatusDone, err)
	}
	return nil
}

// handleDoneStatus takes necessary actions when privileged-command-status annotation switches to "done"
func handleDoneStatus(pc *privilegeCmdController, oldPodResource *v1.Pod, newPodResource *v1.Pod, requestSpec *requestSpec) error {
	// Sleep to sync with the plugin
	time.Sleep(time.Second)

	// Delete the privileged pod
	glog.Infof("[%s] Deleting privilege pod", requestSpec.reqID)
	err := deletePod(pc.client, CmdArgs.Namespace, requestSpec)
	if err != nil {
		return err
	}

	// Delete privileged-command-status and privileged-command-container Annotation
	err = deletePrivilegedCommandExecutorAnnotation(pc.client, newPodResource.Namespace, newPodResource, requestSpec)
	if err != nil {
		return fmt.Errorf("failed to delete annotations %s, %s and %s: %s", constants.AnnotationExecuteStatus, constants.AnnotationExecuteContainer, constants.AnnotationExecuteAction, err)
	}
	glog.Infof("[%s] Finished executing command on container %s on pod %s on namespace %s", requestSpec.reqID, newPodResource.Annotations[constants.AnnotationExecuteContainer], newPodResource.Name, newPodResource.Namespace)

	return nil
}

// deletePrivilegedCommandExecutorAnnotation deletes the privileged-command-status and privileged-command-container annotation
func deletePrivilegedCommandExecutorAnnotation(client kubernetes.Interface, namespace string, newPodResource *v1.Pod, requestSpec *requestSpec) error {
	// Delete privileged-command-status and privileged-command-container annotations
	return applyAnnotationDeletionOnPod(client,
		namespace,
		[]string{constants.AnnotationExecuteStatus, constants.AnnotationExecuteContainer, constants.AnnotationExecuteAction},
		newPodResource,
		requestSpec,
	)
}

// updatePrivilegedCommandExecutorAnnotation update privileged-command-status annotation with corresponding status
func updatePrivilegedCommandExecutorAnnotation(client kubernetes.Interface, namespace string, newPodResource *v1.Pod, status string, requestSpec *requestSpec) error {
	// Update privileged-command-status annotation with the new status supplied
	m := make(map[string]string)
	m[constants.AnnotationExecuteStatus] = status
	return applyAnnotationUpdateOnPod(client,
		namespace,
		m,
		newPodResource,
		requestSpec,
	)
}
