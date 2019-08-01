// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang/glog"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

// applyPatchForAnnotation patches annotations onto the pod. It is a helper function for applyAnnotationUpdateOnPod and applyAnnotationDeletionOnPod
func applyPatchForAnnotation(client kubernetes.Interface, namespace string, newPod []byte, modifiedPod []byte, newPodResource *v1.Pod, requestSpec *requestSpec) error {
	// Apply a strategic merge patch between the new and the newly annotated pod
	// Example:
	// Annotations: { "pcc.k8s.yahoo.com/privilege-command-status": "active", "other-annotations": "other-value" }
	// addOrUpdateAnnotations: { "pcc.k8s.yahoo.com/privilege-command-status": "in-progress", "new-annotation": "new-value" }
	// Result: { "privilege-command-status": "in-progress", "other-annotations": "other-value", "new-annotation": "new-value" }
	patch, err := strategicpatch.CreateTwoWayMergePatch(newPod, modifiedPod, v1.Pod{})
	if err != nil {
		return err
	}

	// Patching annotation to the pod
	_, err = client.CoreV1().Pods(namespace).Patch(newPodResource.Name, types.StrategicMergePatchType, patch)
	if err != nil {
		return fmt.Errorf("failed to patch annotation to pod %s: %s", newPodResource.Name, err)
	}
	glog.Infof("[%s] Completed patching annotation on pod %s", requestSpec.reqID, newPodResource.Name)

	return nil
}

// applyAnnotationUpdateOnPod updates annotations on a pod
func applyAnnotationUpdateOnPod(client kubernetes.Interface, namespace string, addOrUpdateAnnotations map[string]string,
	newPodResource *v1.Pod, requestSpec *requestSpec) error {
	newPod, err := json.Marshal(newPodResource)
	if err != nil {
		return err
	}

	// Avoid nil map error
	if newPodResource.Annotations == nil {
		newPodResource.Annotations = map[string]string{}
	}

	// Add or update annotations on pod resource
	for key, value := range addOrUpdateAnnotations {
		newPodResource.Annotations[key] = value
	}

	modifiedPod, err := json.Marshal(newPodResource)
	if err != nil {
		return err
	}

	err = applyPatchForAnnotation(client, namespace, newPod, modifiedPod, newPodResource, requestSpec)
	return err
}

// applyAnnotationDeletionOnPod deletes specified annotations on a pod
func applyAnnotationDeletionOnPod(client kubernetes.Interface, namespace string, deleteAnnotations []string, newPodResource *v1.Pod, requestSpec *requestSpec) error {
	newPod, err := json.Marshal(newPodResource)
	if err != nil {
		return err
	}

	// Delete annotations from pod resource
	for _, key := range deleteAnnotations {
		if newPodResource.Annotations[key] == "" {
			return fmt.Errorf("annotation %s to be deleted does not exist", key)
		}
		delete(newPodResource.Annotations, key)
	}

	modifiedPod, err := json.Marshal(newPodResource)
	if err != nil {
		return err
	}

	err = applyPatchForAnnotation(client, namespace, newPod, modifiedPod, newPodResource, requestSpec)
	return err
}

// getNodeName returns the name of the node the pod resides in
func getNodeName(pod *v1.Pod) (string, error) {
	// Find the node name from the specs of the pod
	nodeName := pod.Spec.NodeName
	if nodeName == "" {
		return "", errors.New("no node name detected for target pod: " + pod.Name)
	}
	return nodeName, nil
}
