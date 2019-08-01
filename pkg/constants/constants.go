// Copyright 2019 Oath, Inc.
// Licensed under the terms of the Apache Version 2.0 License. See LICENSE file for terms.
package constants

const (
	// Common annotation names and statuses
	AnnotationExecuteStatus    = "privileged-command-status"    // annotation provided by kubectl plugin for checking status of pod
	AnnotationExecuteContainer = "privileged-command-container" // annotation provided by kubectl plugin for the container name
	AnnotationExecuteAction    = "privileged-command-action"    // annotation provided by kubectl plugin for the action to execute
	StatusActive               = "active"                       // status for privileged-command-status is active
	StatusInProgress           = "in-progress"                  // status for privileged-command-status is in progress
	StatusDone                 = "done"                         // status for privileged-command-status is done
	StatusError                = "error"                        // status for privileged-command-status is error

	// Privilege pod specifications
	PrivilegeContainer = "priv-pod" // container of the privilege pod
)
