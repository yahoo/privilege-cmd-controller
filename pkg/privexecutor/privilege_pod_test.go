// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"fmt"
	"testing"

	guuid "github.com/google/uuid"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// createNode creates node on fake clientset
func createNode(client kubernetes.Interface, nodeName string) error {
	_, err := client.CoreV1().Nodes().Create(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	})
	return err
}

// isPrivileged will return true a pod has any privileged containers
func isPrivileged(pod *v1.Pod) bool {
	for _, c := range pod.Spec.InitContainers {
		if c.SecurityContext == nil || c.SecurityContext.Privileged == nil {
			continue
		}
		if *c.SecurityContext.Privileged {
			return true
		}
	}
	for _, c := range pod.Spec.Containers {
		if c.SecurityContext == nil || c.SecurityContext.Privileged == nil {
			continue
		}
		if *c.SecurityContext.Privileged {
			return true
		}
	}
	return false
}

// TestCreatePrivilegedPod checks if a privileged pod is created upon calling createPrivilegedPod
// with the correct pod name and on the right target node
func TestCreatePrivilegedPod(t *testing.T) {
	client := fake.NewSimpleClientset()
	namespace := "default"
	nodeName := "targetNode"

	currRequest := requestSpec{
		privPodName: "priv-test-pod-target-container",
		reqID:       guuid.New().String(),
	}

	// Create node on client
	err := createNode(client, nodeName)
	if err != nil {
		t.Errorf("Failed to create node %s on fake client: %s", nodeName, err)
	}

	// Create privileged pod on node
	err = createPrivilegedPod(client, namespace, nodeName, &currRequest)
	// The error here should be that pod is not running FakeClient will not allow watching over the status of pod
	expectedError := fmt.Errorf("privileged pod %s is not running after %d seconds, it is currently in %s phase", currRequest.privPodName, 3, "")
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error: %s ; Actual error: %s", expectedError, err)
	}

	// Test that the pod created is privileged
	pod, err := client.CoreV1().Pods(namespace).Get(currRequest.privPodName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error in retrieving pod: %s", err)
	}

	if pod.Name != currRequest.privPodName {
		t.Errorf("Expected pod name: %s ; Actual pod name: %s", pod.Name, currRequest.privPodName)
	}

	if pod.Spec.NodeName != nodeName {
		t.Errorf("Expected node: %s ; Actual node: %s", nodeName, pod.Spec.NodeName)
	}

	if !isPrivileged(pod) {
		t.Errorf("Found pod is not privileged")
	}
}

func TestCreatePod(t *testing.T) {
	client := fake.NewSimpleClientset()
	namespace := "default"
	nodeName := "targetNode"

	currRequest := requestSpec{
		privPodName: "priv-test-pod-target-container",
		reqID:       guuid.New().String(),
	}

	// Create node on client
	err := createNode(client, nodeName)
	if err != nil {
		t.Errorf("Failed to create node %s on fake client: %s", nodeName, err)
	}

	// Create pod on the node using privileged pod specs
	pod := privilegedPodSpec(currRequest.privPodName, nodeName, namespace)
	err = createPod(client, pod)
	if err != nil {
		t.Errorf("Failed to create pod %s on fake client: %s", currRequest.privPodName, err)
	}

	// Test that the pod created is privileged
	pod, err = client.CoreV1().Pods(namespace).Get(currRequest.privPodName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error in retrieving pod: %s", err)
	}

	if pod.Name != currRequest.privPodName {
		t.Errorf("Expected pod name: %s ; Actual pod name: %s", pod.Name, currRequest.privPodName)
	}

	if pod.Spec.NodeName != nodeName {
		t.Errorf("Expected node: %s ; Actual node: %s", nodeName, pod.Spec.NodeName)
	}

	if !isPrivileged(pod) {
		t.Errorf("Found pod is not privileged")
	}
}

// TestDeletePod tests that deletePod function successfully deletes pod
func TestDeletePod(t *testing.T) {
	client := fake.NewSimpleClientset()
	namespace := "default"

	// Make a fake request with request ID
	currRequest := requestSpec{
		privPodName: "priv-test-pod-target-container",
		reqID:       guuid.New().String(),
	}

	// Create the pod to be deleted
	err := createPod(client, &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      currRequest.privPodName,
			Namespace: namespace,
		},
	})
	if err != nil {
		t.Errorf("Failed to create pod %s on fake client: %s", currRequest.privPodName, err)
	}

	// Test that pod currently exists
	_, err = client.CoreV1().Pods(namespace).Get(currRequest.privPodName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error in retrieving pod: %s", err)
	}

	// Delete the target pod
	err = deletePod(client, namespace, &currRequest)
	if err != nil {
		t.Errorf("Error in deleting pod: %s", err)
	}

	// Test that the pod no longer exists in namespace
	_, err = client.CoreV1().Pods(namespace).Get(currRequest.privPodName, metav1.GetOptions{})
	if err == nil {
		t.Errorf("Pod %s should not exist", currRequest.privPodName)
	}
}
