// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"reflect"
	"testing"

	guuid "github.com/google/uuid"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/yahoo/privilege-cmd-controller/pkg/constants"
)

// getPodSpec creates pod specs for testing
func getPodSpec(nodeName string) *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"init-annotation1": "init-value1",
				"init-annotation2": "init-value2",
			},
		},
		Spec: v1.PodSpec{
			NodeName: nodeName,
		},
	}
}

// TestGetNodeName tests that function getNodeName will return the correct node names
// given a pod object
func TestGetNodeName(t *testing.T) {
	nodeNames := []string{"node1", "node2"}
	var podList []*v1.Pod

	// Create pods with different node names and inject into pod list
	for _, nodeName := range nodeNames {
		podList = append(podList, getPodSpec(nodeName))
	}

	// Construct the test node list which will grab all the node names
	var testNodeList []string
	for _, pod := range podList {
		node, _ := getNodeName(pod)
		testNodeList = append(testNodeList, node)
	}

	// Test that expected and actual arrays are the same
	if !reflect.DeepEqual(testNodeList, nodeNames) {
		t.Errorf("Expected and actual arrays are different. Actual node names: %v. Expected node names: %v", testNodeList, nodeNames)
	}
}

// TestApplyAnnotationChangesOnPod tests that function ApplyAnnotationChangesOnPod will
// correctly update annotations
func TestApplyAnnotationUpdateOnPod(t *testing.T) {
	client := fake.NewSimpleClientset()
	namespace := "default"

	// Create the map of annotation - label
	annotationToAdd := map[string]string{
		constants.AnnotationExecuteContainer: "targetContainer",
		constants.AnnotationExecuteStatus:    "active",
	}

	// Create expected annotations map
	expectedAnnotations := map[string]string{
		"init-annotation1":                   "init-value1",
		"init-annotation2":                   "init-value2",
		constants.AnnotationExecuteContainer: "targetContainer",
		constants.AnnotationExecuteStatus:    "active",
	}

	currRequest := requestSpec{
		privPodName: "priv-test-pod-targetContainer",
		reqID:       guuid.New().String(),
	}

	// Create pod test
	pod, _ := client.CoreV1().Pods(namespace).Create(getPodSpec("targetNode"))
	for podAnnotationKey := range pod.Annotations {
		for a := range annotationToAdd {
			if podAnnotationKey == a {
				t.Errorf("Initial pod contains annotation %s but it should not exist", a)
			}
		}
	}

	// Add annotations to the pod
	err := applyAnnotationUpdateOnPod(client, namespace, annotationToAdd, pod, &currRequest)
	if err != nil {
		t.Errorf("Error applying annotation changes: %s", err)
	}

	// Check that actual and expected maps are the same
	if !reflect.DeepEqual(pod.Annotations, expectedAnnotations) {
		t.Errorf("Actual annotations: %v ; Expected Annotations: %v", pod.Annotations, expectedAnnotations)
	}

	// Update annotation privilege-command-status to in-progress
	annotationToUpdate := map[string]string{
		constants.AnnotationExecuteStatus: "in-progress",
	}

	// Create expected annotations map
	expectedUpdateAnnotations := map[string]string{
		"init-annotation1":                   "init-value1",
		"init-annotation2":                   "init-value2",
		constants.AnnotationExecuteContainer: "targetContainer",
		constants.AnnotationExecuteStatus:    "in-progress",
	}

	// Update annotation to the pod
	err = applyAnnotationUpdateOnPod(client, namespace, annotationToUpdate, pod, &currRequest)
	if err != nil {
		t.Errorf("Error applying annotation changes: %s", err)
	}

	// Check that actual and expected maps are the same
	if !reflect.DeepEqual(pod.Annotations, expectedUpdateAnnotations) {
		t.Errorf("Actual annotations: %v ; Expected Annotations: %v", pod.Annotations, expectedAnnotations)
	}
}

// TestApplyAnnotationChangesOnPod tests that function ApplyAnnotationChangesOnPod will
// correctly delete annotations without changing other annotations / specs of pod
func TestApplyAnnotationDeletionOnPod(t *testing.T) {
	client := fake.NewSimpleClientset()
	namespace := "default"
	annotationToDelete := []string{"init-annotation1"}

	currRequest := requestSpec{
		privPodName: "priv-test-pod-targetContainer",
		reqID:       guuid.New().String(),
	}

	// Create pod test
	pod, _ := client.CoreV1().Pods(namespace).Create(getPodSpec("targetNode"))

	// Create expected annotations map
	expectedAnnotations := map[string]string{
		"init-annotation2": "init-value2",
	}

	// Apply the annotation deletion on the pod
	err := applyAnnotationDeletionOnPod(client, namespace, annotationToDelete, pod, &currRequest)
	if err != nil {
		t.Errorf("Error applying annotation changes: %s", err)
	}

	// Check that actual and expected maps are the same
	if !reflect.DeepEqual(pod.Annotations, expectedAnnotations) {
		t.Errorf("Actual annotations: %v ; Expected Annotations: %v", pod.Annotations, expectedAnnotations)
	}
}
