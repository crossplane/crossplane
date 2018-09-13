# Project Conductor (codename)

Project Conductor brings cloud provider resources into Kubernetes with unifying cloud abstractions.

## Requirements

In order to use the controllers and resources from this repository, you should have all of the following requirements:

| Requirement | Installation | Tested version |
| ----------- | ------------ | -------------- |
| Kubernetes cluster | https://kubernetes.io/docs/setup/ | `v1.10.7` |
| `kubectl` | https://kubernetes.io/docs/tasks/tools/install-kubectl/ | `v1.11.2` |
| `kubebuilder` | https://book.kubebuilder.io/quick_start.html#installation | `1.0.0` `9c4c6c213a8d17f8c21cf4f1aa9cefb99fbbf5ca` |
| `dep` | https://github.com/golang/dep#installation | `v0.4.1` `37d9ea0a` |
| `kustomize` | https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md | `1.0.6` `017c4ae0aa19195db2a51ecc5aa82c56a1f1c99b` |

## Usage

You can start using the custom controllers and resources from Conductor with the following commands:

First build the container image:

```console
make docker-build
```

Then publish the container image to a repository that will be accessible by your testing environment:

```console
make docker-push
```

Finally, you can deploy the custom controllers and resources:

```console
make deploy
```

The controller manager should now be running in the `conductor-system` namespace, if it is not then please refer to the [troubleshooting documentation](./docs/troubleshooting.md).
You can now go ahead and create an instance of a cloud resource.
For example, to create a Google Cloud SQL instance, run the following:

```console
kubectl create -f config/samples/gcp_v1alpha1_cloudsql.yaml
```

You can get status updates and watch progress in the controller manager logs:

```console
kubectl -n conductor-system logs -f $(kubectl -n conductor-system get pod -l control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}')
```

It's also possible to get status and information from the CRD itself.
For example, to get information about a Google Cloud SQL CRD, run the following:

```console
kubectl get cloudsql -o yaml
```

### Scaling the controller manager

You can scale the controller manager `Deployment` down to 0, essentially stopping the controller manager for the time being.
This can be useful if you are working on fixes and don't want it running until you have a new image to use.

```console
kubectl -n conductor-system scale --replicas=0 deployment/conductor-controller-manager
```

Similarly, you can scale it up to 1 to start it running again:

```console
kubectl -n conductor-system scale --replicas=1 deployment/conductor-controller-manager
```

## Developer iteration workflow

The typical developer iteration workflow is to scale the controller manager down, delete any CRD instances, make your changes and then deploy again.
The commands are summarized below, but there is obviously more automation here that can streamline this:

```bash
kubectl -n conductor-system scale --replicas=0 deployment/conductor-controller-manager
kubectl delete -f config/samples/
# update name of cloud sql CRD in config/samples/gcp_v1alpha1_cloudsql.yaml

make docker-build
make docker-push
make deploy

kubectl -n conductor-system scale --replicas=1 deployment/conductor-controller-manager
kubectl -n conductor-system get pod -o wide
kubectl -n conductor-system logs -f $(kubectl -n conductor-system get pod -l control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}')

kubectl create -f config/samples/gcp_v1alpha1_cloudsql.yaml
```

## Cleanup/Teardown

To completely clean up all resources in your target cluster, run the following command:

```console
make clean-deploy
```

## Troubleshooting

For help with any issues using the controllers and resources in this repo, please refer to the
[troubleshooting documentation](./docs/troubleshooting.md).