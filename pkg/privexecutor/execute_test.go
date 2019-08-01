// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"errors"
	"flag"
	"fmt"
	"testing"

	"github.com/yahoo/privilege-cmd-controller/pkg/constants"

	guuid "github.com/google/uuid"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// init initializes the flag variables for all tests
func init() {
	var _ = flag.String("privilegePodImage", "docker.ouroath.com:4443/yahoo-cloud/priv-cmd-exec-util:latest", "Image for the privileged pod to be created on target node. It contains all related privileged command utilities.")
	var _ = flag.String("namespace", "kube-pcc", "Namespace for privileged pod to be scheduled")
	var _ = flag.String("serviceaccount", "kube-priv-pod", "Service account for privileged pod to be scheduled")
	var _ = flag.Int("privPodTimeout", 3, "Timeout for checking running status of the privilege pod in seconds") // set to 3s for testing
	flag.Parse()
}

// construct target container
var container = v1.Container{
	Name:            "target-container",
	Image:           "image",
	ImagePullPolicy: v1.PullAlways,
}

// construct explicitly the status of the target container
// with container ID
var containerStatus = v1.ContainerStatus{
	Name:        "target-container",
	ContainerID: "docker://containerid",
}

// getPodSpec creates pod specs for testing
var targetPodSpecWithNode = &v1.Pod{
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
		NodeName:   "targetNode",
		Containers: []v1.Container{container},
	},
	Status: v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{containerStatus},
	},
}

// TestProcess tests the Process function in execute.go
// It tests the following:
// 1. Checking if it creates privilege pod when annotation switches to "active"
// 2. Checking if it deletes privilege pod and annotations when annotation switches to "done"
func TestProcess(t *testing.T) {
	CmdArgs.PrivPodTimeout = 3
	CmdArgs.Image = "image"
	CmdArgs.Namespace = "default"
	currRequest := requestSpec{
		privPodName: "priv-test-pod-target-container",
		reqID:       guuid.New().String(),
	}
	client := fake.NewSimpleClientset()
	privilegeCmdController := &privilegeCmdController{
		client: client,
	}
	namespace := "default"
	newPod := targetPodSpecWithNode
	oldPod := newPod.DeepCopy() // old pod captures the state of the pod prior to any changes

	_ = applyAnnotationUpdateOnPod(client,
		namespace,
		map[string]string{
			constants.AnnotationExecuteStatus:    constants.StatusActive,
			constants.AnnotationExecuteContainer: "target-container",
			constants.AnnotationExecuteAction:    "target-action",
		},
		newPod,
		&currRequest,
	)

	// The annotation should be active, so handleActiveStatus is called
	// The error here should be that pod is not running FakeClient will not allow watching over the status of pod
	err := Process(privilegeCmdController, oldPod, newPod, &currRequest)
	expectedError := fmt.Errorf("unable to act upon annotation %s change to %s: privileged pod priv-test-pod-target-container is not running after 3 seconds, it is currently in  phase", constants.AnnotationExecuteStatus, constants.StatusActive)
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error: %s ; Actual error: %s", expectedError, err)
	}

	// Retrieve pod from client and ensure that it has been created in the correct node with the correct name
	pod, err := client.CoreV1().Pods(namespace).Get("priv-test-pod-target-container", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving privilege pod: %s", err)
	}

	if pod.Spec.NodeName != "targetNode" {
		t.Errorf("Expected node: %s ; Actual node: %s", "targetNode", pod.Spec.NodeName)
	}

	if pod.Name != "priv-test-pod-target-container" {
		t.Errorf("Expected pod name: %s ; Actual pod name: %s", "priv-test-pod-target-container", pod.Name)
	}

	// Explicitly set privilege-command-status annotation to done, as the previous process errors out before
	// annotation can be updated
	err = updatePrivilegedCommandExecutorAnnotation(client, namespace, newPod, constants.StatusDone, &currRequest)
	if err != nil {
		t.Errorf("Unable to update annotation of pod %s to done: %s", newPod.Name, err)
	}

	// The annotation should now be done, so handleDoneStatus is called
	err = Process(privilegeCmdController, oldPod, newPod, &currRequest)
	if err != nil {
		t.Errorf("Error processing done status: %s", err)
	}

	_, err = client.CoreV1().Pods("").Get("priv-test-pod-target-container", metav1.GetOptions{})
	if !k8serrors.IsNotFound(err) {
		t.Error("Pod \"priv-test-pod-target-container\" should not exist")
	}
	if newPod.Annotations[constants.AnnotationExecuteContainer] != "" || newPod.Annotations[constants.AnnotationExecuteStatus] != "" {
		t.Errorf("Annotations %s or %s have not been deleted", constants.AnnotationExecuteContainer, constants.AnnotationExecuteStatus)
	}
}

// TestHandleActiveStatus checks that the controller properly handles setting privileged-command-status to active
func TestHandleActiveStatus(t *testing.T) {
	// Explicitly set CmdArgs variables as flag variables cannot be read by tests
	CmdArgs.PrivPodTimeout = 3
	CmdArgs.Image = "image"
	CmdArgs.Namespace = "default"

	// Set current request with a request ID
	currRequest := requestSpec{
		privPodName: "priv-test-pod-target-container",
		reqID:       guuid.New().String(),
	}

	// Create fake client
	client := fake.NewSimpleClientset()
	privilegeCmdController := &privilegeCmdController{
		client: client,
	}
	namespace := "default"
	newPod := targetPodSpecWithNode
	oldPod := newPod.DeepCopy() // old pod captures the state of the pod prior to any changes

	// Explicitly annotate newPod with privileged-command-status and privileged-command-container
	_ = applyAnnotationUpdateOnPod(client,
		namespace,
		map[string]string{
			constants.AnnotationExecuteStatus:    constants.StatusActive,
			constants.AnnotationExecuteContainer: "target-container",
		},
		newPod,
		&currRequest,
	)

	// The error here should be that pod is not running FakeClient will not allow watching over the status of pod
	err := handleActiveStatus(privilegeCmdController, oldPod, newPod, &currRequest)
	expectedError := errors.New("privileged pod priv-test-pod-target-container is not running after 3 seconds, it is currently in  phase")
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error: %s ; Actual error: %s", expectedError, err)
	}

	// Target pod should have its annotation changed to in-progress
	if newPod.Annotations[constants.AnnotationExecuteStatus] != constants.StatusInProgress {
		t.Errorf("Expected label for annotation %s: %s ; Actual label: %s", constants.AnnotationExecuteStatus, constants.StatusInProgress, newPod.Annotations[constants.AnnotationExecuteStatus])
	}

	// Retrieve pod from client and ensure that it has been created in the correct node with the correct name
	pod, err := client.CoreV1().Pods(namespace).Get("priv-test-pod-target-container", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving privilege pod: %s", err)
	}

	if pod.Spec.NodeName != "targetNode" {
		t.Errorf("Expected node: %s ; Actual node: %s", "targetNode", pod.Spec.NodeName)
	}

	if pod.Name != "priv-test-pod-target-container" {
		t.Errorf("Expected pod name: %s ; Actual pod name: %s", "priv-test-pod-target-container", pod.Name)
	}
}

// TestHandleActiveStatus checks that the controller properly handles setting privileged-command-status to done
func TestHandleDoneStatus(t *testing.T) {
	// Explicitly set CmdArgs variables as flag variables cannot be read by tests
	CmdArgs.PrivPodTimeout = 3
	CmdArgs.Image = "image"
	CmdArgs.Namespace = "default"

	// Set current request with a request ID
	currRequest := requestSpec{
		privPodName: "priv-test-pod-target-container",
		reqID:       guuid.New().String(),
	}

	// Create fake client
	client := fake.NewSimpleClientset()
	privilegeCmdController := &privilegeCmdController{
		client: client,
	}
	namespace := "default"
	newPod := targetPodSpecWithNode
	oldPod := newPod.DeepCopy() // old pod captures the state of the pod prior to any changes

	// Explicitly create privileged pod that handleActiveStatus creates
	privPod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "priv-test-pod-target-container",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "targetNode",
		},
	}
	_, err := client.CoreV1().Pods(namespace).Create(privPod)
	if err != nil {
		t.Error("Unable to create privilege pod")
	}

	// Explicitly annotate newPod with privileged-command-status and privileged-command-container
	_ = applyAnnotationUpdateOnPod(client,
		namespace,
		map[string]string{
			constants.AnnotationExecuteStatus:    constants.StatusDone,
			constants.AnnotationExecuteContainer: "target-container",
			constants.AnnotationExecuteAction:    "target-action",
		},
		newPod,
		&currRequest,
	)

	// Call handleDoneStatus
	err = handleDoneStatus(privilegeCmdController, oldPod, newPod, &currRequest)
	if err != nil {
		t.Errorf("Error handling done status: %s", err)
	}

	// Check that the privilege pod no longer exists
	_, err = client.CoreV1().Pods("").Get("priv-test-pod-target-container", metav1.GetOptions{})
	if !k8serrors.IsNotFound(err) {
		t.Error("Pod \"priv-test-pod-target-container\" should not exist")
	}

	// Check that the annotations have been deleted
	if newPod.Annotations[constants.AnnotationExecuteContainer] != "" || newPod.Annotations[constants.AnnotationExecuteStatus] != "" {
		t.Errorf("Annotations %s or %s have not been deleted", constants.AnnotationExecuteContainer, constants.AnnotationExecuteStatus)
	}
}
