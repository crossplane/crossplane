# OAM POC Workflow
* Owner: Dan Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Defunct

This document describes a basic workflow for utilizing OAM types as implemented
in [1276](https://github.com/crossplane/crossplane/pull/1276). The controllers
described are for an initial POC and will be moved out of core Crossplane in
later iterations. For a complete picture of the long-term implementation, take a
look at the [OAM Runtime Architecture
doc](https://docs.google.com/document/d/11AYNGhvry_B3l_tO3Yyv1f9tDoezZOpGIYV56oZwb5s/edit#heading=h.x458enhyy1sq).

## 1. Crossplane Installation

The following OAM CRDs will be installed with Crossplane:

- `ApplicationConfiguration`
- `Component`
- `TraitDefinition`
- `ScopeDefinition`
- `WorkloadDefinition`
- `ContainerizedWorkload`

Crossplane will also create a `WorkloadDefinition` *instance* that looks as
follows:

```yaml
apiVersion: core.oam.dev/v1alpha2
kind: WorkloadDefinition
metadata:
  name: containerizedworkloads.core.oam.dev
spec:
  definitionRef:
    name: containerizedworkloads.core.oam.dev
```

This makes the `ContainerizedWorkload` core workload type available to be used
by OAM `Components`. Other workload types can be created and made available by
creating a CRD for the workload and a `WorkloadDefinition` that references it.

The OAM-related controllers will also be started on installation:

- **Application Configuration controller**: watches `ApplicationConfiguration`
  resources and creates the corresponding workloads based on the inline
  parameters and referenced `Components`
- **Containerized Workload controller**: watches `ContainerizedWorkload`
  resources and packages them into `KubernetesApplications` for scheduling to a
  target cluster 

## 2. Component Creation

Users create a `Component` to define a unit of deployment that can be included
in an `ApplicationConfiguration`. A `Component` workload; a custom resource with
a corresponding `WorkloadDefinition`.

```yaml
apiVersion: core.oam.dev/v1alpha2
kind: Component
metadata:
  name: myapp
spec:
  workload:
    apiVersion: core.oam.dev/v1alpha2
    kind: ContainerizedWorkload
    metadata:
      name: my-workload
    spec:
      osType: linux
      containers:
      - name: tbs11-app
        image: hasheddan/tbs11:latest
        resources:
          cpu:
            required: 1.0
          memory:
            required: 100MB
        env:
          - name: DB_HOST
            valueFrom:
              secretKeyRef:
                name: mysqlconn
                key: endpoint
          - name: DB_USER
            valueFrom:
              secretKeyRef:
                name: mysqlconn
                key: username
          - name: DB_PASSWORD
            valueFrom:
              secretKeyRef:
                name: mysqlconn
                key: password
        ports:
          - containerPort: 8080
  parameters: 
  - name: imageName
    required: false
    fieldPaths: 
    - "spec.containers[0].image"
```

The creation of this `Component` does not trigger reconciliation for any
Crossplane controller.


## 3. Application Configuration Creation

In order to deploy the `Component` described above, a user must create an
`ApplicationConfiguration` that references it.

```yaml
apiVersion: core.oam.dev/v1alpha2
kind: ApplicationConfiguration
metadata:
  name: myapp-dev
spec:
  components:
    - componentName: myapp
      parameterValues:
        - name: imageName
          value: hello-world:latest
```

The creation of an `ApplicationConfiguration` controller will queue a reconcile
for its controller. The controller will create instances of the workload defined
in each of the `Components` in `spec.Component`. In this example, a single
`ContainerizedWorkload` will be created with the following configuration:

```yaml
apiVersion: core.oam.dev/v1alpha2
kind: ContainerizedWorkload
metadata:
  name: myapp-workload
spec:
  osType: linux
  containers:
  - name: tbs11-app
    image: hello-world:latest # the ApplicationConfiguration controller replaced this value based on the parameters in its spec
    resources:
      cpu:
        required: 1.0
      memory:
        required: 100MB
    env:
      - name: DB_HOST
        valueFrom:
          secretKeyRef:
            name: mysqlconn
            key: endpoint
      - name: DB_USER
        valueFrom:
          secretKeyRef:
            name: mysqlconn
            key: username
      - name: DB_PASSWORD
        valueFrom:
          secretKeyRef:
            name: mysqlconn
            key: password
    ports:
      - containerPort: 8080
```

The creation of the `ContainerizedWorkload` will also queue a reconcile of its
controller. The controller will take the `ContainerizedWorkload` and package it
into a `KubernetesApplication` with the following configuration:

```yaml
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplication
metadata:
  name: myapp-workload-0003
spec:
  resourceSelector:
    matchLabels:
      app: 2a23de82-58e4-11ea-8e2d-0242ac130003 # UID of the ContainerizedWorkload 
  targetSelector:
    matchLabels: # TODO: pass down scheduling labels?
  resourceTemplates:
    - metadata:
        name: myappworkload-deployment
        labels:
           app: 2a23de82-58e4-11ea-8e2d-0242ac130003
      spec:
        template:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            namespace: default
            name: myapp-workload
            labels:
               app: 2a23de82-58e4-11ea-8e2d-0242ac130003
          spec:
            selector:
              matchLabels:
                 app: 2a23de82-58e4-11ea-8e2d-0242ac130003
            template:
              metadata:
                labels:
                   app: 2a23de82-58e4-11ea-8e2d-0242ac130003
              spec:
                - name: tbs11-app
                  image: hello-world:latest
                  resources:
                    cpu:
                      required: 1.0
                    memory:
                      required: 100MB
                  env:
                    - name: DB_HOST
                      valueFrom:
                        secretKeyRef:
                          name: mysqlconn
                          key: endpoint
                    - name: DB_USER
                      valueFrom:
                        secretKeyRef:
                          name: mysqlconn
                          key: username
                    - name: DB_PASSWORD
                      valueFrom:
                        secretKeyRef:
                          name: mysqlconn
                          key: password
                  ports:
                    - containerPort: 8080
```

From this point forward, the `KubernetesApplication` will be scheduled according
to the existing behavior of its controllers.