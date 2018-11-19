# Crossplane Local Deployment and Test

The Local Framework is used to run end to end and integration tests on Crossplane. The framework depends on a running instance of Kubernetes.
The framework also provides scripts for starting Kubernetes using `minikube` so users can
quickly spin up a Kubernetes cluster.The Local framework is designed to install Crossplane, run tests, and uninstall Crossplane.

## Requirements
1. Docker version => 1.2 && < 17.0
2. Ubuntu 16 (the framework has only been tested on this version)
3. Kubernetes with kubectl configured
4. Crossplane

## Instructions

## Setup

### Install Kubernetes
You can choose any Kubernetes flavor of your choice.  The test framework only depends on kubectl being configured.
The framework also provides scripts to install Kubernetes.

- **Minikube** (recommended for MacOS): Run [minikube.sh](./minikube.sh) to setup a single-node Minikube Kubernetes.
  - Minikube v0.28.2 and higher is supported

#### Minikube (recommended for MacOS)
Starting the cluster on Minikube is as simple as running:
```console
cluster/local/minikube.sh up
```

To copy Crossplane image generated from your local build into the Minikube VM, run the following commands after `minikube.sh up` succeeded:
```
cluster/local/minikube.sh helm-install
```

Stopping the cluster and destroying the Minikube VM can be done with:
```console
cluster/local/minikube.sh clean
```

For complete list of subcommands supported by `minikube.sh`, run:
```console
cluster/local/minikube.sh
```

## Run Tests
From the project root do the following:
#### 1. Build crossplane:
```
make build
```

#### 2. Start Kubernetes
```
cluster/local/minikube.sh up
```

#### 3. Install Crossplane
```
cluster/local/minikube.sh helm-install
```

#### 4. Interact with Crossplane
Use `kubectl` to create and/or delete external resources CRD's in your Minikube cluster

#### 5. Uninstall Crossplane
```
cluster/local/minikube.sh helm-delete
```  

#### 6. Stop Kubernetes
```
cluster/local/minikube.sh down
```
