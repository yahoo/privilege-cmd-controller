// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package privexecutor

import (
	"flag"
	"fmt"
	"strings"

	"github.com/golang/glog"
	guuid "github.com/google/uuid"
	"github.com/yahoo/privilege-cmd-controller/pkg/constants"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// cmdArgs collects variables from command line arguments
type cmdArgs struct {
	PrivPodTimeout int
	Namespace      string
	Image          string
	Serviceaccount string
}

// privilegeCmdController contains necessary variables for making client calls
type privilegeCmdController struct {
	Controller cache.Controller
	client     kubernetes.Interface
	restConfig *rest.Config
}

var (
	//CmdArgs initializes a global cmdArgs variable
	CmdArgs cmdArgs
)

// NewPrivilegeCmdController returns a privilegeCmdController struct with the controller and client
func NewPrivilegeCmdController(client kubernetes.Interface, restConfig *rest.Config) *privilegeCmdController {
	glog.Info("Initiating new privilege command controller")

	// Collect command line arguments
	CmdArgs.PrivPodTimeout = flag.Lookup("privPodTimeout").Value.(flag.Getter).Get().(int)
	CmdArgs.Namespace = flag.Lookup("namespace").Value.(flag.Getter).Get().(string)
	CmdArgs.Image = flag.Lookup("privilegePodImage").Value.(flag.Getter).Get().(string)
	CmdArgs.Serviceaccount = flag.Lookup("serviceaccount").Value.(flag.Getter).Get().(string)

	// Initialize privilegeCmdController object
	privilegeCmdController := &privilegeCmdController{
		client:     client,
		restConfig: restConfig,
	}

	// Construct the controller object for privilegeCmdController
	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				// Setting namespace to "" will get all pods from all namespaces
				return client.CoreV1().Pods("").List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.CoreV1().Pods("").Watch(options)
			},
		},
		&v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: privilegeCmdController.handleUpdate,
		},
	)

	privilegeCmdController.Controller = controller
	return privilegeCmdController
}

// handleUpdate handles updates to the privileged-command-status annotation
func (pc *privilegeCmdController) handleUpdate(oldObj interface{}, newObj interface{}) {
	oldPodResource := oldObj.(*v1.Pod)
	newPodResource := newObj.(*v1.Pod)

	// Second constraint ensures that all the annotations exist
	if newPodResource.Annotations != nil &&
		newPodResource.Annotations[constants.AnnotationExecuteContainer] != "" &&
		newPodResource.Annotations[constants.AnnotationExecuteAction] != "" &&
		newPodResource.Annotations[constants.AnnotationExecuteStatus] != "" {
		for podAnnotationKey := range newPodResource.Annotations {
			if podAnnotationKey == constants.AnnotationExecuteStatus {
				// Construct the privilege pod name
				privPodName := fmt.Sprintf("priv_%s_%s", newPodResource.Name, newPodResource.Annotations[constants.AnnotationExecuteContainer])
				privPodName = strings.Replace(privPodName, "_", "-", -1)
				privPodName = strings.ToLower(privPodName)

				// Construct request specs for the current request
				currRequestSpec := requestSpec{
					privPodName: privPodName,
					reqID:       guuid.New().String(),
				}

				// Set up the handler
				handler := Process
				err := handler(pc, oldPodResource, newPodResource, &currRequestSpec)

				// Handle any errors from executing privileged command
				// Delete privilege pod, update annotation to error and log the error on the controller
				if err != nil {
					glog.Errorf("[%s] Error on update request on pod %s: %v", currRequestSpec.reqID, newPodResource.Name, err)

					// Delete privileged pod
					err = deletePod(pc.client, CmdArgs.Namespace, &currRequestSpec)
					if err != nil {
						glog.Errorf("[%s] Failure to delete pod after error: %s", currRequestSpec.reqID, err)
					}

					// Update privileged-command-status annotation to error
					// Annotations to be deleted on the plugin side
					err = updatePrivilegedCommandExecutorAnnotation(pc.client, newPodResource.Namespace, newPodResource, constants.StatusError, &currRequestSpec)
					if err != nil {
						glog.Errorf("[%s] Failure to update annotation after error: %s", currRequestSpec.reqID, err)
					}
				}
			}
		}
	}
}
