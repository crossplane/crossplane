---
title: Using Rook Services
toc: true
weight: 450
indent: true
---
# Deploying Yugastore with Rook

This user guide will walk you through [Yugastore] application deployment using
Crossplane's [Rook] stack to run [YugabyteDB] in a Google Cloud [GKE] Kubernetes
cluster. To do so, we will broadly:

1. Provision a GKE Kubernetes cluster
1. Install the Rook [Yugabyte operator] into the GKE cluster
1. Provision a YugabyteDB cluster and deploy the Yugastore app into the GKE cluster

... all using Crossplane!

## Table of Contents

1. [Pre-requisites](#pre-requisites)
2. [Preparation](#preparation)
3. [Set Up Resource Classes](#set-up-resource-classes)
4. [Provision GKE Cluster and Install Rook Yugabyte Operator](#provision-gke-cluster-and-install-rook-yugabyte-operator)
5. [Deploy Yugastore alongside YugabyteDB](#deploy-yugastore-alongside-yugabytedb)
6. [Cleanup](#cleanup)
7. [Conclusion and Next Steps](#conclusion-and-next-steps)

## Pre-requisites

These tools are required to complete this guide. They must be installed on your
local machine.

* [kubectl][install-kubectl]
    * kubectl also be installed using the Azure CLI: `az aks install-cli`
* [Helm][using-helm], minimum version `v2.10.0+`.


## Preparation

This guide assumes that you have an existing Kubernetes cluster, which will
serve as the Crossplane control cluster. Good options for running local
Kubernetes clusters include [KIND] and [Minikube].

In order to utilize GCP services, we must set the `PROJECT_ID` of GCP project we
want to use. Run the following:
```bash
export PROJECT_ID=crossplane-playground # the project that all resources reside.
```

### Set Up Crossplane

Using your local Kubernetes cluster:

1. Install Crossplane from alpha channel. (See the [Crossplane Installation
   Guide][crossplane-install] for more information.)

```bash
helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

2. Install the GCP stack into Crossplane. (See the [GCP stack
   section][gcp-stack-install] of the install guide for more information.)

```bash
cat > stack-gcp.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: gcp
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: stack-gcp
  namespace: gcp
spec:
  package: "crossplane/stack-gcp:v0.2.0"
EOF

kubectl apply -f stack-gcp.yaml
```

3. Install the Rook stack into Crossplane (See the [Rook stack
   section][rook-stack-install] of the install guide for more information.)

```bash
cat > stack-rook.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: rook
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: stack-rook
  namespace: rook
spec:
  package: "crossplane/stack-rook:v0.2.0"
EOF

kubectl apply -f stack-rook.yaml
```

4. Obtain GCP credentials. (See the [Cloud Provider Credentials][cloud-creds]
   docs for more information.)

#### GCP Provider

Using the service account json `crossplane-gcp-provider-key.json` that you
acquired from GCP:

* Generate Base64 encoded value to store in a `Secret`:

  ```bash
  export BASE64ENCODED_GCP_PROVIDER_CREDS=$(base64 crossplane-gcp-provider-key.json | tr -d "\n")
  ```

  * Define a GCP `Provider` (using the `PROJECT_ID` environment variable we set
    above) and `Secret`:

    ```bash
    cat > gcp-provider.yaml <<EOF
    ---
    apiVersion: v1
    data:
      credentials.json: $BASE64ENCODED_GCP_PROVIDER_CREDS
    kind: Secret
    metadata:
      name: gcp-provider-creds
      namespace: crossplane-system
    type: Opaque
    ---
    apiVersion: gcp.crossplane.io/v1alpha2
    kind: Provider
    metadata:
      name: gcp-provider
    spec:
      credentialsSecretRef:
        name: gcp-provider-creds
        namespace: crossplane-system
        key: credentials.json
      projectID: $PROJECT_ID
    EOF

    kubectl apply -f gcp-provider.yaml
    unset BASE64ENCODED_GCP_PROVIDER_CREDS # we don't need this anymore.
    ```

* Verify GCP provider was successfully registered by the crossplane

  ```bash
  kubectl get providers.gcp.crossplane.io
  kubectl -n crossplane-system get secrets
  ```

#### Rook Provider

Rook differs from traditional cloud provider stacks in that it does not come
with a Rook `Provider` custom resource. The reason for this is that Rook runs in
any Kubernetes cluster. Therefore, it utilizes the general Kubernetes `Provider`
custom resource that is packaged as part of the core Crossplane installation. We
will see how to use this `Provider` type further along in this guide.

## Set Up Resource Classes

In order to dynamically provision resources, we need to create resources classes that contain details about how the resources should be provisioned. For Yugastore, we will need resource classes that are capable of fulfilling a `KubernetesCluster` claim and a `PostgreSQLInstance` claim:

```bash
cat > classes.yaml <<EOF
apiVersion: compute.gcp.crossplane.io/v1alpha2
kind: GKEClusterClass
metadata:
  name: standard-gke
  labels:
    app: yugastore
specTemplate:
  machineType: n1-standard-1
  numNodes: 1
  zone: us-central1-b
  providerRef:
    name: gcp-provider
  reclaimPolicy: Delete
---
apiVersion: database.rook.crossplane.io/v1alpha1
kind: YugabyteClusterClass
metadata:
  name: yuga-cluster
specTemplate:
  providerRef:
    name: yugastore-k8s-provider
  reclaimPolicy: Delete
  forProvider:
    name: hello-ybdb-cluster
    namespace: rook-yugabytedb
    master:
      # Replica count for Master.
      replicas: 3
      # Mentioning network ports is optional. If some or all ports are not specified, then they will be defaulted to below-mentioned values, except for tserver-ui.
      network:
        ports:
          - name: yb-master-ui
            port: 7000          # default value
          - name: yb-master-rpc
            port: 7100          # default value
      # Volume claim template for Master
      volumeClaimTemplate:
        metadata:
          name: datadir
        spec:
          accessModes: [ "ReadWriteOnce" ]
          resources:
            requests:
              storage: 1Gi
          storageClassName: standard
    tserver:
      # Replica count for TServer
      replicas: 3
      # Mentioning network ports is optional. If some or all ports are not specified, then they will be defaulted to below-mentioned values, except for tserver-ui.
      # For tserver-ui a cluster ip service will be created if the yb-tserver-ui port is explicitly mentioned. If it is not specified, only StatefulSet & headless service will be created for TServer. TServer ClusterIP service creation will be skipped. Whereas for Master, all 3 kubernetes objects will always be created.
      network:
        ports:
          - name: yb-tserver-ui
            port: 9000
          - name: yb-tserver-rpc
            port: 9100          # default value
          - name: ycql
            port: 9042          # default value
          - name: yedis
            port: 6379          # default value
          - name: ysql
            port: 5433          # default value
      # Volume claim template for TServer
      volumeClaimTemplate:
        metadata:
          name: datadir
        spec:
          accessModes: [ "ReadWriteOnce" ]
          resources:
            requests:
              storage: 1Gi
          storageClassName: standard
EOF

kubectl apply -f classes.yaml
```

The `GKEClusterClass` is relatively straightforward in that it configures a
`GKECluster` and utilizes our previously created GCP `Provider` for connection.
The `YugabyteClusterClass` is less clear. Starting with the provider, we
reference a `Provider` that does not currently exist. Because resource classes
only store configuration data, this is okay as long as the provider exists when
the class is referenced by a claim. As previously mentioned, this provider will
be a Kubernetes `Provider` which we will create after the `GKECluster` is
created and its connection secret is propagated.

The `forProvider` section of the `YugabyteClusterClass` also differs somewhat
from other resource classes. While resource classes like `GKEClusterClass`
specify configuration for a 3rd party API, `YugabyteClusterClass` specifies
configuration for a Kubernetes [CustomResourceDefinition] (CRD) instance in a
target cluster. When the `YugabyteClusterClass` is used to create a
`YugabyteCluster` managed resource in the Crossplane control cluster, the Rook
stack reaches out to the target Kubernetes cluster using the Kubernetes
`Provider` referenced above and creates a Rook `YBCluster` [instance]. The stack
trusts that the CRD kind has been installed in the target cluster and it will fail
to provision the resource it has not (more on this below).

## Provision GKE Cluster and Install Rook Yugabyte Operator

Now that our classes have been created, we need to provision the GKE cluster by
creating a `KubernetesCluster` claim.

```bash
cat > k8sclaim.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: yugastore-app
  labels:
    app: yugastore
---
apiVersion: kubernetes.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: yugastore-k8s-provider
  labels:
    app: yugastore
spec:
  credentialsSecretRef:
    name: yugastore-k8s-secret
    namespace: yugastore-app
---
apiVersion: compute.crossplane.io/v1alpha1
kind: KubernetesCluster
metadata:
  name: yugastore-k8s
  namespace: yugastore-app
  labels:
    app: yugastore
spec:
  writeConnectionSecretToRef:
    name: yugastore-k8s-secret
EOF

kubectl apply -f k8sclaim.yaml
```

Here we create a namespace `yugastore-app` for our Yugastore namespaced
resources to use and also create a Kubernetes `Provider` the references the
secret propagated by the `KubernetesCluster` claim. If you look back at the
`YugabyteClusterClass` we created above, this `yugastore-k8s-provider` is
referenced, so once the secret is propagated, the Rook stack will be able to use
it to provision a `YugabyteCluster`. However, before we get to that, we need to
deploy the Rook Yugabyte operator into the Kubernetes cluster.

```bash
cat > rook-operator.yaml <<EOF
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplication
metadata:
  name: rook-yugabyte
  namespace: gcp-infra-dev
  labels:
    app: yugastore
spec:
  resourceSelector:
    matchLabels:
      app: rook-yugabyte
  clusterSelector:
    matchLabels:
      app: yugastore
  resourceTemplates:
  - metadata:
      name: rook-namespace
      labels:
        operator: rook-yugabyte
    spec:
      template:
        apiVersion: v1
        kind: Namespace
        metadata:
          name: rook-yugabytedb-system
  - metadata:
      name: rook-app-namespace
      labels:
        operator: rook-yugabyte
    spec:
      template:
        apiVersion: v1
        kind: Namespace
        metadata:
          name: rook-yugabytedb
          labels:
            app: yugastore
  - metadata:
      name: rook-crds
      labels:
        operator: rook-yugabyte
    spec:
      template:
        apiVersion: apiextensions.k8s.io/v1beta1
        kind: CustomResourceDefinition
        metadata:
          name: ybclusters.yugabytedb.rook.io
        spec:
          group: yugabytedb.rook.io
          names:
            kind: YBCluster
            listKind: YBClusterList
            singular: ybcluster
            plural: ybclusters
          scope: Namespaced
          version: v1alpha1
  - metadata:
      name: rook-clusterrole
      labels:
        operator: rook-yugabyte
    spec:
      template:
        apiVersion: rbac.authorization.k8s.io/v1beta1
        kind: ClusterRole
        metadata:
          name: rook-yugabytedb-operator
        rules:
        - apiGroups:
          - ""
          resources:
          - pods
          verbs:
          - get
          - list
        - apiGroups:
          - ""
          resources:
          - services
          verbs:
          - get
          - list
          - watch
          - create
          - update
          - patch
          - delete
        - apiGroups:
          - apps
          resources:
          - statefulsets
          verbs:
          - get
          - list
          - watch
          - create
          - update
          - patch
          - delete
        - apiGroups:
          - yugabytedb.rook.io
          resources:
          - "*"
          verbs:
          - "*"
  - metadata:
      name: rook-serviceaccount
      labels:
        operator: rook-yugabyte
    spec:
      template:
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: rook-yugabytedb-operator
          namespace: rook-yugabytedb-system
  - metadata:
      name: rook-serviceaccount
      labels:
        operator: rook-yugabyte
    spec:
      template:
        apiVersion: rbac.authorization.k8s.io/v1beta1
        kind: ClusterRoleBinding
        metadata:
          name: rook-yugabytedb-operator
          namespace: rook-yugabytedb-system
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: rook-yugabytedb-operator
        subjects:
        - kind: ServiceAccount
          name: rook-yugabytedb-operator
          namespace: rook-yugabytedb-system
  - metadata:
      name: rook-serviceaccount
      labels:
        operator: rook-yugabyte
    spec:
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: rook-yugabytedb-operator
          namespace: rook-yugabytedb-system
          labels:
            app: rook-yugabytedb-operator
        spec:
          selector:
            matchLabels:
              app: rook-yugabytedb-operator
          replicas: 1
          template:
            metadata:
              labels:
                app: rook-yugabytedb-operator
            spec:
              serviceAccountName: rook-yugabytedb-operator
              containers:
              - name: rook-yugabytedb-operator
                image: rook/yugabytedb:master
                args: ["yugabytedb", "operator"]
                env:
                - name: POD_NAME
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.name
                - name: POD_NAMESPACE
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.namespace
EOF

kubectl apply -f rook-operator.yaml
```

While this is quite a large set of configuration, all it is doing is taking the
Rook Yugabyte [operator YAML] and packaging it into a Crossplane
`KubernetesApplication` resource so that we can deploy it into our newly created
GKE cluster.

!! TODO: check successful provisioning (to be updated after CRD defs are finalized) !! 

## Deploy Yugastore alongside YugabyteDB

Now that we have a GKE cluster up and running, we can create our YugabyteDB cluster and install Yugastore alongside it.

```bash
cat > yugastore.yaml <<EOF
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: yugastore-db
  namespace: yugastore-app
  labels:
    app: yugastore
spec:
  writeConnectionSecretToRef:
    name: yugastore-db-secret
---
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplication
metadata:
  name: yugastore
  namespace: yugastore-app
  labels:
    app: yugastore
spec:
  resourceSelector:
    matchLabels:
      app: yugastore
  clusterSelector:
    matchLabels:
      app: yugastore
  resourceTemplates:
  - metadata:
      name: yugastore-namespace
      labels:
        app: yugastore
    spec:
      template:
        apiVersion: v1
        kind: Namespace
        metadata:
          name: rook-yugastore
          labels:
            app: yugastore
  - metadata:
      name: yugastore-deployment
      labels:
        app: yugastore
    spec:
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          namespace: rook-yugastore
          name: yugastore
          labels:
            app: yugastore
        spec:
          selector:
            matchLabels:
              app: yugastore
          template:
            metadata:
              labels:
                app: yugastore
            spec:
              containers:
                - name: yugastore
                  image: yugabyte/yugastore:latest # Pending merge of https://github.com/yugabyte/yugastore/pull/4
                  imagePullPolicy: Always
                  command: ["/usr/local/yugastore/bin/start-for-crossplane.sh"]
                  env:
                  - name: DB_HOST
                    value: "yb-tserver-hello-ybdb-cluster-1.yb-tservers-hello-ybdb-cluster.rook-yugabytedb.svc.cluster.local"
                  - name: APP_HOST
                    value: "localhost"
                  - name: APP_PORT
                    value: "3001"
                  ports:
                    - containerPort: 3001
                      name: yugastore
  - metadata:
      name: yugastore-service
      labels:
        app: yugastore
    spec:
      template:
        apiVersion: v1
        kind: Service
        metadata:
          namespace: rook-yugastore
          name: yugastore
          labels:
            app: yugastore
        spec:
          ports:
            - port: 3001
          selector:
            app: yugastore
          type: LoadBalancer
EOF

kubectl apply -f yugastore.yaml
```

!! TODO: check successful provisioning (to be updated after CRD defs are finalized) !! 
!! TODO: make sure loadbalancer IP address is propagated to `KubernetesApplicationResource` `yugastore-service` !!

## Cleanup



## Conclusion and Next Steps

In this guide we:

* Setup a local Kubernetes cluster with Crossplane, stack-gcp, and stack-rook installed
* Provisioned a GKE Kubernetes cluster
* Installed the Rook Yugabyte operator into the GKE cluster
* Created a YugabyteDB cluster in the GKE cluster
* Deployed Yugastore to the GKE cluster, using the YugabyteDB cluster as its database

If you would like to try out a similar workflow using a different cloud
provider, take a look at the other [services guides][services]. If you would like
to learn more about stacks, checkout the [stacks guide][stacks]

<!-- Named links -->
[Yugastore]: https://github.com/yugabyte/yugastore
[Rook]: https://rook.io/
[Yugabyte operator]: https://rook.io/docs/rook/v1.1/yugabytedb.html
[YugabyteDB]: https://www.yugabyte.com/
[GKE]: https://cloud.google.com/kubernetes-engine/

[KIND]: https://kind.sigs.k8s.io/
[Minikube]: https://github.com/kubernetes/minikube

[install-kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[using-helm]: https://docs.helm.sh/using_helm/

[crossplane-install]: ../install-crossplane.md#alpha
[gcp-stack-install]: ../install-crossplane.md#gcp-stack
[rook-stack-install]: ../install-crossplane.md#rook-stack
[cloud-creds]: ../cloud-providers/gcp/gcp-provider.md

[CustomResourceDefinition]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[instance]: https://rook.io/docs/rook/v1.1/yugabytedb-cluster-crd.html
[operator YAML]: https://github.com/rook/rook/blob/master/cluster/examples/kubernetes/yugabytedb/operator.yaml

[services]: ../services-guide.md
[stacks]: ../stacks-guide.md
