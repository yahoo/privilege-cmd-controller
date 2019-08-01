// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/yahoo/privilege-cmd-controller/pkg/constants"
	"k8s.io/apimachinery/pkg/fields"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// createPrivilegedPod creates a new privileged kubernetes pod on target node
func createPrivilegedPod(client kubernetes.Interface, namespace string, nodeName string, requestSpec *requestSpec) error {
	glog.Infof("[%s] Creating privileged pod %s in node %s under namespace %s", requestSpec.reqID, requestSpec.privPodName, nodeName, namespace)
	// Specify the privileged pod to be created inside target node
	pod := privilegedPodSpec(requestSpec.privPodName, nodeName, namespace)
	err := createPod(client, pod)
	if err != nil {
		return err
	}

	// Create a watcher that watches over the pod with same name
	watch, err := client.CoreV1().Pods(namespace).Watch(metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", requestSpec.privPodName).String(),
	})
	if err != nil {
		return errors.New("error starting watcher for privilege pod: " + err.Error())
	}

	// Check that privileged pod is in running phase. If it has not started running after set timeout, raise error
	glog.Infof("[%s] Waiting for privilege pod %s to be in a running status", requestSpec.reqID, requestSpec.privPodName)
	err = func() error {
		for {
			select {
			case event := <-watch.ResultChan():
				p, _ := event.Object.(*v1.Pod)
				if p.Status.Phase == v1.PodRunning {
					glog.Infof("[%s] Privileged pod %s is running", requestSpec.reqID, p.Name)
					return nil
				}
			case <-time.After(time.Duration(CmdArgs.PrivPodTimeout) * time.Second):
				p, _ := client.CoreV1().Pods(namespace).Get(requestSpec.privPodName, metav1.GetOptions{})
				return fmt.Errorf("privileged pod %s is not running after %d seconds, it is currently in %s phase", p.Name, CmdArgs.PrivPodTimeout, p.Status.Phase)
			}
		}
	}()
	watch.Stop()

	if err != nil {
		return err
	}

	return nil
}

// createPod creates specified pod in target namespace
func createPod(client kubernetes.Interface, pod *v1.Pod) error {
	// Create the privileged pod in the targeted namespace
	_, err := client.CoreV1().Pods(pod.Namespace).Create(pod)
	if err != nil {
		return fmt.Errorf("failed to create pod %s on node %s: %s", pod.Name, pod.Spec.NodeName, err)
	}
	return nil
}

// deletePod deletes the privileged kubernetes pod specified by podName
func deletePod(client kubernetes.Interface, namespace string, requestSpec *requestSpec) error {
	glog.Infof("[%s] Deleting pod %s under namespace %s", requestSpec.reqID, requestSpec.privPodName, namespace)
	propagationPolicy := metav1.DeletePropagationForeground
	err := client.CoreV1().Pods(namespace).Delete(requestSpec.privPodName, &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete pod %s: %s", requestSpec.privPodName, err)
	}
	return nil
}

// privilegedPodSpecification is the specification for the privileged pod to be created on target node
func privilegedPodSpec(podName, nodeName, namespace string) *v1.Pod {
	// TypeMeta specification for privileged pod
	typeMetadata := metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	}

	// ObjectMeta specification for privileged pod
	objectMetadata := metav1.ObjectMeta{
		Name:      podName,
		Namespace: namespace,
	}

	// Pod spec specifications for privileged pod
	// with volumeMount, container,
	volumeMounts := []v1.VolumeMount{{
		Name:      "docker-sock",
		MountPath: "/var/run/docker.sock",
	}}

	privileged := true
	privilegedContainer := v1.Container{
		Name:            constants.PrivilegeContainer,
		Image:           CmdArgs.Image,
		ImagePullPolicy: v1.PullAlways,

		SecurityContext: &v1.SecurityContext{
			Privileged: &privileged,
		},
		VolumeMounts: volumeMounts,
	}

	hostPathType := v1.HostPathFile
	volumeSources := v1.VolumeSource{
		HostPath: &v1.HostPathVolumeSource{
			Path: "/var/run/docker.sock",
			Type: &hostPathType,
		},
	}

	podSpecs := v1.PodSpec{
		ServiceAccountName: CmdArgs.Serviceaccount,
		HostPID:            true,
		NodeName:           nodeName,
		RestartPolicy:      v1.RestartPolicyNever,
		Containers:         []v1.Container{privilegedContainer},
		Volumes: []v1.Volume{{
			Name:         "docker-sock",
			VolumeSource: volumeSources,
		}},
	}

	pod := v1.Pod{
		TypeMeta:   typeMetadata,
		ObjectMeta: objectMetadata,
		Spec:       podSpecs,
	}
	return &pod
}
