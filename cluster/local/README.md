# Crossplane Local Deployment and Test

The Local Framework is used to run end to end and integration tests on Crossplane.
The framework depends on a running instance of Kubernetes. The framework also
provides scripts for starting a local Kubernetes using `kind`, `minikube` and `microk8s`
so users can quickly spin up a Kubernetes cluster.The Local framework is designed
to install Crossplane, run tests, and uninstall Crossplane.

## Instructions

## Setup

### Install Kubernetes
You can choose any Kubernetes flavor of your choice.  The test framework only
depends on kubectl being configured. The framework also provides scripts to install Kubernetes.

- **kind**: Run [kind.sh](./kind.sh) to setup a single-node Minikube Kubernetes.
  - kind v0.8.0 and higher is supported
- **Minikube**: Run [minikube.sh](./minikube.sh) to setup a single-node Minikube Kubernetes.
  - Minikube v0.28.2 and higher is supported
- **MicroK8s**: Run [microk8s.sh](./microk8s.sh) to setup a single-node Microk8s Kubernetes.
  - MicroK8s v1.14 (move from dockerd to containerd) and higher is supported

#### Kind
Starting the cluster on kind is as simple as running:
```console
cluster/local/kind.sh up
```
You can check the default Kubernetes version in the script and override it.

To copy Crossplane image generated from your local build into the local registry in
the kind cluster and install it via Helm, run the following commands after
`kind.sh up` succeeded:
```console
cluster/local/kind.sh helm-install
```

Stopping the cluster and destroying the kind cluster can be done with:
```console
cluster/local/kind.sh clean
```

For complete list of subcommands supported by `kind.sh`, run:
```console
cluster/local/kind.sh
```

#### Minikube
Starting the cluster on Minikube is as simple as running:
```console
cluster/local/minikube.sh up
```

To copy Crossplane image generated from your local build into the Minikube VM, run the following commands after `minikube.sh up` succeeded:
```console
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

#### MicroK8s
Starting the cluster on MicroK8s is as simple as running:
```console
cluster/local/microk8s.sh up
```

To copy Crossplane image generated from your local build into the MicroK8s container registry, run the following commands after `microk8s.sh up` succeeded:
```console
cluster/local/microk8s.sh helm-install
```

Resetting the MicroK8s cluster can be done with:
```console
cluster/local/microk8s.sh clean
```

Stopping the MicroK8s cluster can be done with:
```console
cluster/local/microk8s.sh down
```

For complete list of subcommands supported by `microk8s.sh`, run:
```console
cluster/local/microk8s.sh
```

## Run locally out-of-cluster

For convenience and speed of development, it can be a good option to run
crossplane locally, out-of-cluster. To do that, there is a target in the
Makefile:

```
make run
```

For preserving the logs, something like the following command could be used:

```console
make run 2>&1 | tee -a local-log
```

If running crossplane locally out-of-cluster, it is important to make
sure crossplane is not also running in-cluster, because the two
crossplanes could interfere with each other. This can be done by either
deleting or scaling down the `crossplane` deployment in the namespace
`crossplane-system`.

## Run Tests
The following sections provide commands helpful for development environments.
`kind.sh` may be replaced with [`local.sh`](./local.sh), [`microk8s.sh`](./microk8s.sh),
or [`minikube.sh`](./minikube.sh) in those environments. These commands expect to be
run from the project root.

#### 1. Build crossplane
```
make build
```

#### 2. Start Kubernetes
```
cluster/local/kind.sh up
```

#### 3. Install Crossplane
```
cluster/local/kind.sh helm-install
```

#### 4. Interact with Crossplane
Use `kubectl` to create and/or delete external resources CRD's in your Minikube cluster

#### 5. Run tests locally
```
make test
```

#### 6. Uninstall Crossplane
```
cluster/local/kind.sh helm-delete
```  

#### 7. Stop Kubernetes
```
cluster/local/kind.sh down
```

## Run Integration Tests
In addition to the manual testing procedure described above which gives some
flexibility on interacting with the cluster, one could also test the integration
of the built artifacts with the cluster in a single command. This will create a
new Kubernetes cluster using `kind` tool, and then installs Crossplane and checks
a few assertions, and finally destructs the cluster.

#### 1. Build crossplane
```
make build
```

#### 2. Run Integration Tests
```
make e2e
```

This step is also included in [CI workflow](../../INSTALL.md#ci-workflow-and-options).
