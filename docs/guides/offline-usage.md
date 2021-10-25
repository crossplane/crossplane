---
title: Offline Usage
toc: true
weight: 280
indent: true
---

# Offline Usage

The following guide shows on how to install Crossplane for offline usage. It has been originally developed for a workshop with no internet access but can possibly reused for other setups.

Use-case: As a developer, I would like to play around with Crossplane MRs and XRs by using AWS as an example. I would like to have everything on my machine to install Crossplane while I'm offline. I'd especially like to start over once I'm stuck.

## Perquisites 
Software to be installed before going offline: kubectl, Docker, helm, Kind, svn, curl, jq
* Crossplane CLI
* Docs: `svn export https://github.com/crossplane/crossplane.git/trunk/docs`

## Prerequisites artifacts
The following scripts, images, and charts need to be pulled before going offline:
```console
docker pull kindest/node:v1.21.1
docker pull registry
docker pull gcr.io/google-samples/hello-app:1.0

docker pull crossplane/crossplane:v1.4.1
helm pull crossplane-stable/crossplane # downloads crossplane-1.4.1.tgz into the current dir
docker pull crossplane/provider-aws:master

curl -O https://kind.sigs.k8s.io/examples/kind-with-registry.sh
chmod +x kind-with-registry.sh
```

## When offline

### Kind cluster

Create a Kind cluster with a local container registry 

```console
# install kind cluster with local registry
./kind-with-registry.sh

# check the registry and the kind cluster is running:
docker ps
CONTAINER ID   IMAGE                  COMMAND                  CREATED          STATUS          PORTS                       NAMES
72f2ee72148b   kindest/node:v1.21.1   "/usr/local/bin/entr…"   14 minutes ago   Up 14 minutes   127.0.0.1:56117->6443/tcp   kind-control-plane
8195d8a17344   registry:2             "/entrypoint.sh /etc…"   39 minutes ago   Up 39 minutes   127.0.0.1:5000->5000/tcp    kind-registry

kubectl get pods --all-namespaces
NAMESPACE            NAME                                         READY   STATUS    RESTARTS   AGE
kube-system          coredns-558bd4d5db-5v6cd                     1/1     Running   0          19s
kube-system          coredns-558bd4d5db-b56ts                     1/1     Running   0          19s
kube-system          etcd-kind-control-plane                      1/1     Running   0          27s
kube-system          kindnet-9cz9t                                1/1     Running   0          19s
kube-system          kube-apiserver-kind-control-plane            1/1     Running   0          27s
kube-system          kube-controller-manager-kind-control-plane   1/1     Running   0          36s
kube-system          kube-proxy-vmw24                             1/1     Running   0          19s
kube-system          kube-scheduler-kind-control-plane            1/1     Running   0          27s
local-path-storage   local-path-provisioner-547f784dff-xt495      1/1     Running   0          19s

# check the network is there with the containers kind-registry and kind-control-plane
docker inspect kind | jq .[].Containers
...

# push the images to the local registry
docker tag crossplane/crossplane:v1.4.1 localhost:5000/crossplane/crossplane:v1.4.1
docker push localhost:5000/crossplane/crossplane:v1.4.1
docker tag crossplane/provider-aws:master localhost:5000/crossplane/provider-aws:master
docker push localhost:5000/crossplane/provider-aws:master

# check the images have been pushed
curl http://localhost:5000/v2/_catalog
{"repositories":["crossplane/crossplane","crossplane/provider-aws"]}
```

### Crossplane
```console
# install crossplane:
helm install crossplane --namespace crossplane-system --create-namespace crossplane-1.4.1.tgz --set image.repository=localhost:5000/crossplane/crossplane

# check deployed status:
helm list -n crossplane-system

# check installed CRDs
kubectl get crds
NAME                                                       
compositeresourcedefinitions.apiextensions.crossplane.io
compositionrevisions.apiextensions.crossplane.io
compositions.apiextensions.crossplane.io
configurationrevisions.pkg.crossplane.io
configurations.pkg.crossplane.io
controllerconfigs.pkg.crossplane.io
locks.pkg.crossplane.io
providerrevisions.pkg.crossplane.io
providers.pkg.crossplane.io

# check installed pods
kubectl get pods -n crossplane-system
NAME                                      READY   STATUS    RESTARTS   AGE
crossplane-6f974db97-cr57k                1/1     Running   0          9m47s
crossplane-rbac-manager-dd8d65f77-rqr6r   1/1     Running   0          9m47s

```

### Provider
```console
# install provider
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: "crossplane/provider-aws:master"
EOF
```

# TODO Error when offline
```
kubectl describe providers provider-aws
...
Warning  UnpackPackage  20s (x2 over 20s)  packages/provider.pkg.crossplane.io  cannot unpack package: failed to fetch package digest from remote: Get "https://index.docker.io/v2/": dial tcp: lookup index.docker.io on 10.96.0.10:53: no such host

```


## Cleanup
```console
kind delete clusters kind
helm delete crossplane --namespace crossplane-system
docker kill $(docker ps -q -f name=kind-registry)
docker rm $(docker ps -a -q -f name=kind-registry)
docker network rm $(docker network ls -q -f name=kind)
```

## Links: 
* https://kind.sigs.k8s.io/docs/user/working-offline/
* https://kind.sigs.k8s.io/docs/user/local-registry/
* https://hub.docker.com/_/registry/
* https://github.com/kubernetes/enhancements/blob/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry/README.md

## Notes
annotate the node. via https://docs.openfaas.com/tutorials/local-kind-registry/ 
kubectl annotate node kind-control-plane "kind.x-k8s.io/registry=localhost:5000"
