---
title: Run Applications
toc: true
weight: 5
indent: true
---

# Run Applications

Crossplane strives to be the best Kubernetes add-on to provision and manage the
infrastructure and services your applications need directly from kubectl. A huge
part of this mission is arriving at an elegant, flexible way to model and manage
cloud native applications. Crossplane allows your team to deploy and run
applications using the [Open Application Model] (OAM).

OAM is a team-centric model for cloud native apps. Like Crossplane, OAM focuses
on the different people who might be involved in the deployment of a cloud
native application. In this getting started guide:

* _Infrastructure Operators_ provide the infrastructure applications need.
* _Application Developers_ build and supply the components of an application.
* _Application Operators_ compose, deploy, and run application configurations.

We'll play the roles of each of these team members as we deploy an application -
Service Tracker - that consists of several services. One of these services, the
`data-api`, is backed by a managed PostgreSQL database that is provisioned
on-demand by Crossplane.

![Service Tracker Diagram]

> This guide follows on from the previous one in which we covered defining,
> composing, and [publishing infrastructure]. You'll need to have defined and
> published a PostgreSQLInstance with at least one working Composition in order
> to create the OAM application we'll use in this guide.

## Install the OAM Addon

As the infrastructure operator our work is almost done - we defined, published,
and composed the infrastructure  that our application developer and operator
team-mates will use in the previous guide. One task remains for us, which is to
install and configure Crossplane's OAM addon.

<ul class="nav nav-tabs">
<li class="active"><a href="#install-tab-helm3" data-toggle="tab">Helm 3</a></li>
<li><a href="#install-tab-helm2" data-toggle="tab">Helm 2</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="install-tab-helm3" markdown="1">
Use Helm 3 to install the latest official `alpha` release of Crossplane OAM
Addon, suitable for community use and testing:

```console
kubectl create namespace crossplane-system
helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install addon-oam-kubernetes-local --namespace crossplane-system crossplane-alpha/oam-core-resources
```

> Note that the OAM Addon requires at least Kubernetes 1.16.

</div>
<div class="tab-pane fade" id="install-tab-helm2" markdown="1">
Use Helm 2 to install the latest official `alpha` release of the Crossplane OAM
Addon, suitable for community use and testing:

```console
kubectl create namespace crossplane-system
helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name addon-oam-kubernetes-local --namespace crossplane-system crossplane-alpha/oam-core-resources
```

> Note that the OAM Addon requires at least Kubernetes 1.16.

</div>
</div>

```yaml
---
apiVersion: core.oam.dev/v1alpha2
kind: WorkloadDefinition
metadata:
  name: containerizedworkloads.core.oam.dev
spec:
  definitionRef:
    name: containerizedworkloads.core.oam.dev
```

Once the addon is installed we must define the [_workloads_] and [_traits_] that
our platform supports. OAM applications consist of workloads, each of which may
be modified by traits. The infrastructure operator may choose which workloads
and traits their platform supports by creating or deleting `WorkloadDefinitions`
and `TraitDefinitions`. We'll discuss workloads and traits in more detail below.
Run the following command to add support for the workloads and traits required
by this guide:

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/run/definitions.yaml`
```

## Publish Application Components

Now we'll play the role of the application developer. Our Service Tracker
application consists of a UI service, four API services, and a PostgreSQL
database. Under the Open Application Model application developers define
[_components_] that application operators may compose into applications, which
produce workloads. Creating components allows us as application developers to
communicate any fundamental, suggested, or optional properties of our services
and their infrastructure requirements.

```yaml
---
apiVersion: core.oam.dev/v1alpha2
kind: Component
metadata:
  name: data-api-database
spec:
  workload:
    apiVersion: database.example.org/v1alpha1
    kind: PostgreSQLInstanceRequirement
    metadata:
      name: app-postgresql
    spec:
      parameters:
        storageGB: 20
      compositionSelector:
        matchLabels:
          guide: quickstart
  parameters:
  - name: database-secret
    description: Secret to which to write PostgreSQL database connection details.
    required: true
    fieldPaths:
    - spec.writeConnectionSecretToRef.name
  - name: database-provider
    description: |
      Cloud provider that should be used to create a PostgreSQL database.
      Either alibaba, aws, azure, or gcp.
    fieldPaths:
    - spec.compositionSelector.matchLabels.provider
---
apiVersion: core.oam.dev/v1alpha2
kind: Component
metadata:
  name: data-api
spec:
  workload:
    apiVersion: core.oam.dev/v1alpha2
    kind: ContainerizedWorkload
    metadata:
      name: data-api
    spec:
      containers:
        - name: data-api
          image: artursouza/rudr-data-api:0.50
          env:
            - name: DATABASE_USER
              fromSecret:
                key: username
            - name: DATABASE_PASSWORD
              fromSecret:
                key: password
            - name: DATABASE_HOSTNAME
              fromSecret:
                key: endpoint
            - name: DATABASE_NAME
              value: postgres
            - name: DATABASE_PORT
              value: "5432"
            - name: DATABASE_DRIVER
              value: postgresql
          ports:
            - name: http
              containerPort: 3009
              protocol: TCP
          livenessProbe:
            exec:
              command: [wget, -q, http://127.0.0.1:3009/status, -O, /dev/null, -S]
  parameters:
    - name: database-secret
      description: Secret from which to read PostgreSQL connection details.
      required: true
      fieldPaths:
        - spec.containers[0].env[0].fromSecret.name
        - spec.containers[0].env[1].fromSecret.name
        - spec.containers[0].env[2].fromSecret.name
---
apiVersion: core.oam.dev/v1alpha2
kind: Component
metadata:
  name: flights-api
spec:
  workload:
    apiVersion: core.oam.dev/v1alpha2
    kind: ContainerizedWorkload
    metadata:
      name: flights-api
    spec:
      containers:
        - name: flights-api
          image: sonofjorel/rudr-flights-api:0.49  
          env:
            - name: DATA_SERVICE_URI
          ports:
            - name: http
              containerPort: 3003
              protocol: TCP
  parameters:
    - name: data-uri
      description: URI at which the data service is serving
      required: true
      fieldPaths:
      - spec.containers[0].env[0].value
---
apiVersion: core.oam.dev/v1alpha2
kind: Component
metadata:
  name: quakes-api
spec:
  workload:
    apiVersion: core.oam.dev/v1alpha2
    kind: ContainerizedWorkload
    metadata:
      name: quakes-api
    spec:
      containers:
        - name: quakes-api
          image: sonofjorel/rudr-quakes-api:0.49
          env:
            - name: DATA_SERVICE_URI
          ports:
            - name: http
              containerPort: 3012
              protocol: TCP
  parameters:
    - name: data-uri
      description: URI at which the data service is serving
      required: true
      fieldPaths:
      - spec.containers[0].env[0].value
---
apiVersion: core.oam.dev/v1alpha2
kind: Component
metadata:
  name: service-tracker-ui
spec:
  workload:
    apiVersion: core.oam.dev/v1alpha2
    kind: ContainerizedWorkload
    metadata:
      name: web-ui
    spec:
      containers:
        - name: service-tracker-ui
          image: sonofjorel/rudr-web-ui:0.49
          env:
            - name: FLIGHT_API_ROOT
            - name: WEATHER_API_ROOT
            - name: QUAKES_API_ROOT
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
  parameters:
    - name: flights-uri
      description: URI at which the flights service is serving
      required: true
      fieldPaths:
      - spec.containers[0].env[0].value
    - name: weather-uri
      description: URI at which the weather service is serving
      required: true
      fieldPaths:
      - spec.containers[0].env[1].value
    - name: quakes-uri
      description: URI at which the quakes service is serving
      required: true
      fieldPaths:
      - spec.containers[0].env[2].value
---
apiVersion: core.oam.dev/v1alpha2
kind: Component
metadata:
  name: weather-api
spec:
  workload:
    apiVersion: core.oam.dev/v1alpha2
    kind: ContainerizedWorkload
    metadata:
      name: weather-api
    spec:
      containers:
        - name: weather-api
          image: sonofjorel/rudr-weather-api:0.49
          env:
            - name: DATA_SERVICE_URI
          ports:
            - name: http
              containerPort: 3015
              protocol: TCP
  parameters:
    - name: data-uri
      description: URI at which the data service is serving
      required: true
      fieldPaths:
      - spec.containers[0].env[0].value
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/run/components.yaml`
```

Each of the above components describes a particular kind of workload. The
Service Tracker application consists of two kinds of workload:

* A [`ContainerizedWorkload`] is a long-running containerized process.
* A `PostSQLInstanceRequirement` is a PostgreSQL instance and database.

All OAM components configure a kind of workload, and any kind of Kubernetes
resource may act as an OAM workload as long as an infrastructure operator has
allowed it to by authoring a `WorkloadDefinition`.

## Run The Application

Finally, we'll play the role of an application operator and tie together the
application components and infrastructure that our application developer and
infrastructure operator team-mates have published. In OAM this is done by
authoring an [_application configuration_]:

```yaml
apiVersion: core.oam.dev/v1alpha2
kind: ApplicationConfiguration
metadata:
  name: service-tracker
spec:
  components:
    - componentName: data-api-database
      parameterValues:
        - name: database-secret
          value: tracker-database-secret
    - componentName: data-api
      parameterValues:
        - name: database-secret
          value: tracker-database-secret
    - componentName: flights-api
      parameterValues:
        - name: data-uri
          value: "http://data-api.default.svc.cluster.local:3009/"
      traits:
        - trait:
            apiVersion: core.oam.dev/v1alpha2
            kind: ManualScalerTrait
            metadata:
              name: flights-api
            spec:
              replicaCount: 2
    - componentName: quakes-api
      parameterValues:
        - name: data-uri
          value: "http://data-api.default.svc.cluster.local:3009/"  
      traits:
        - trait:
            apiVersion: core.oam.dev/v1alpha2
            kind: ManualScalerTrait
            metadata:
              name: quakes-api
            spec:
              replicaCount: 2
    - componentName: weather-api
      parameterValues:
        - name: data-uri
          value: "http://data-api.default.svc.cluster.local:3009/"
      traits:
        - trait:
            apiVersion: core.oam.dev/v1alpha2
            kind: ManualScalerTrait
            metadata:
              name: weather-api
            spec:
              replicaCount: 2
    - componentName: service-tracker-ui
      parameterValues:
        - name: flights-uri
          value: "http://flights-api.default.svc.cluster.local:3003/"
        - name: weather-uri
          value: "http://weather-api.default.svc.cluster.local:3015/"
        - name: quakes-uri
          value: "http://quakes-api.default.svc.cluster.local:3012/"
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/run/appconfig.yaml`
```

This application configuration names each of components the application
developer created earlier to produce workloads. The application operator may (or
in some cases _must_) provide parameter values for a component in order to
specify certain configuration values. Component parameters represent
configuration settings that the component author - the application developer -
deemed to be of interest to application operators.

```yaml
- componentName: data-api-database
  parameterValues:
  - name: database-provider
    value: azure
```

> If you created Compositions for more than one provider in the previous guide
> you can add the above parameter to the `data-api-database` component to choose
> which cloud provider the Service Tracker's database should run on.

You might notice that some components include a [`ManualScalerTrait`]. Traits
augment the workload produced by a component with additional features, allowing
application operators to make decisions about the configuration of a component
without having to involve the developer. The `ManualScalerTrait` allows an
application operator to specify how many replicas should exist of any scalable
kind of workload.

> Note that the OAM spec also includes the concept of an _application scope_.
> Crossplane does not yet support scopes.

## Use The Application

Finally, we'll open and use the Service Tracker application we just deployed.

<ul class="nav nav-tabs">
<li class="active"><a href="#connect-tab-lb" data-toggle="tab">Load Balancer</a></li>
<li><a href="#connect-tab-forward" data-toggle="tab">Port Forward</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="connect-tab-lb" markdown="1">

If you deployed Service Tracker to a managed cluster like AKS, ACK, EKS, or GKE
with support for load balancer Services you should be able to browse to the IP
of the `web-ui` service on port 8080 - for example <http://10.0.0.1:8080>.

```console
kubectl get svc web-ui -o=jsonpath={.status.loadBalancer.ingress[0].ip}
```

</div>
<div class="tab-pane fade" id="connect-tab-forward" markdown="1">

If you're using a cluster that doesn't support load balancer Services, like Kind
or Minikube you can use a port forward instead, and connect to
<http://localhost:8080.>

```console
kubectl port-forward deployment/web-ui 8080
```

</div>
</div>

You should see the Service Tracker dashboard in your browser. Hit 'Refresh Data'
for each of the services to ensure the Service Tracker web UI can connect to its
various data API services and populate its PostgreSQL database:

![Service Tracker Dashboard]

If everything was successful you should be able to browse to Flights,
Earthquakes, or Weather to see what's going on in the world today:

![Service Tracker Flights]

## Clean Up

To shut down your application, simply run:

```console
kubectl delete applicationconfiguration service-tracker
```

If you also wish to delete the components, workload definitions, and trait
definitions we created in this guide, run:

```console
kubectl delete -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/run/components.yaml`
kubectl delete -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/run/definitions.yaml`
```

[Open Application Model]: https://oam.dev/
[publishing infrastructure]: publish-infrastructure.md
[Service Tracker Diagram]: run-applications-diagram.jpg
[_workloads_]: https://github.com/oam-dev/spec/blob/1.0.0-alpha2/3.workload.md
[_traits_]: https://github.com/oam-dev/spec/blob/1.0.0-alpha2/6.traits.md
[_components_]: https://github.com/oam-dev/spec/blob/1.0.0-alpha2/4.component.md
[_application configuration_]: https://github.com/oam-dev/spec/blob/1.0.0-alpha2/7.application_configuration.md
[`ContainerizedWorkload`]: https://github.com/oam-dev/spec/blob/1.0.0-alpha2/core/workloads/containerized_workload/containerized_workload.md
[`ManualScalerTrait`]: https://github.com/oam-dev/spec/blob/1.0.0-alpha2/core/traits/manual_scaler_trait.md
[_application scope_]: https://github.com/oam-dev/spec/blob/1.0.0-alpha2/5.application_scopes.md
[Service Tracker Dashboard]: run-applications-dash.png
[Service Tracker Flights]: run-applications-flights.png
