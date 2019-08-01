# Privilege Command Controller for Kubernetes

> Privilege Command Controller, based on the Controller pattern of Kubernetes, enables the users to execute privileged commands on their non-privileged containers using the `kubectl` command line tool.

## Table of Contents
- [Background](#background)
- [Architecture](#architecture)
- [Install](#install)
- [Usage](#usage)
- [Security](#security)
- [API](#api)
- [Maintainers](#maintainers)
- [Contribute](#contribute)
- [License](#license)

## Background
Privileged commands are those that require access to the host to make privileged system calls. For example, the container wishes to use UNIX capabilities like manipulating network or using ptrace privilege for commands like `gdb`, `gcore`, then the container must be privileged. 

However, Kubernetes containers are non-privileged by default because granting privileges to containers pose security risks and they can be exploited to impact the host. 

This means that users will not be able to debug their containers because debugging tools like `gdb` or `gcore` require `ptrace` UNIX privilege. On the other hand, for Kubernetes admin, it is unsafe to grant users privileged containers because privileged containers can be exploited to abuse the host system. 

Privileged Command Controller provides a simple solution to this issue through `kubectl` plugins.

## Architecture
Privilege Command Controller has four core components: 
- Controller: The controller watches for changes in annotations and triggers creation of privilege pod and execution of privileged actions
- Plugin (`kubectl priv-cmd-exec`): The plugin applies annotations which triggers the controller to create a privilege pod on target node and execute privileged actions. It is also responsible for streaming output back to the user
- Privilege Pod: Running a privilege pod means that it will be able to make privileged system calls directly to the host. This privilege pod is created under admin-controlled namespace and is inaccessible by other users. It is controlled by Privilege Command Controller
- nsenter: [nsenter](http://man7.org/linux/man-pages/man1/nsenter.1.html) is shorthand for “namespace enter”, and it allows the privilege pod to mount to all namespaces (such as pid, ipc, utc, net) of the target container. This ensures that the privilege commands do not affect the host system, and will at most impact only the target container

The following is a pictorial representation of Privilege Command Controller: 
<p align="center">
  <img src=img/priv-cmd-controller.png>
</p>

## Install

## Usage
Privilege Command Controller binary can be configured using the following flags: 
```
  -c, --kubeconfig string          Path to a kubeconfig file
  -n, --namespace string           Namespace for privileged pod to be scheduled (default "kube-pcc")
  -t, --privPodTimeout int         Timeout for checking running status of the privilege pod in seconds (default 300)
  -i, --privilegePodImage string   Image for the privileged pod to be created on target node. It contains all related privileged command utilities. (default "")
  -s, --serviceaccount string      Service account for privileged pod to be scheduled (default "kube-priv-pod")
```

## Security

## API

## Maintainers
Core Team : omega-core@verizonmedia.com

## Contribute
Please refer to the [contributing file](Contributing.md) for information about how to get involved. We welcome issues, questions, and pull requests.

## License

Copyright 2019 Oath Inc. Licensed under the Apache License, Version 2.0 (the "License")
