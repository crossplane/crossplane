# Deploying Crossplane Locally

This directory contains several scripts that automate common local development
flows for Crossplane, allowing you to deploy your local build of Crossplane to
a `kind`, `minikube`, or `microk8s` cluster.

* Run [kind.sh](./kind.sh) to setup a single-node kind Kubernetes cluster. kind
  v0.8.0 and higher is supported.
* Run [minikube.sh](./minikube.sh) to setup a single-node Minikube Kubernetes
  cluster. Minikube v0.28.2 and higher is supported.
* Run [microk8s.sh](./microk8s.sh) to setup a single-node Microk8s Kubernetes
  cluster. MicroK8s v1.14 and higher is supported.
