# Performance Characteristics of Providers

* Owner: Sergen Yalçın (@sergenyalcin)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

There are multiple implementations of providers with different characteristics in regards to provision speed and resource 
utilization. However, there is currently no method to measure the performance characteristics of these different 
implementations. These characteristic results are determinant for deciding some architectural topics and implementation 
details.

In the previous measurement issues, various data collection and reporting tools/methods were used. At the same time, 
different data sets were formed in each test regarding the collected metrics. Considering the purpose of the test and 
the approaches of the tester, it is normal to see such differences. However, in the context of measuring the performance 
metrics of providers, at least for the some basic metrics (such as CPU/Memory Utilization), capturing a common method 
and collecting & reporting them through a generic tool will be an important step that will simplify and accelerate the 
processes. Therefore, we will develop a tool that makes measurements and reporting over a determined set of metrics. 
Comparing the different implementations of providers will be easier because of the consistent tooling.

## Goals

A program has many measurable metrics in terms of performance. This is no different in the context of our providers. It 
will be possible to collect dozens of metrics under many titles and to make performance-related inferences from them. 
However, we will limit the scope of the generic tool to be developed to the metrics listed below:

- Average CPU Utilization
- Peak CPU Utilization
- Average Memory Utilization
- Peak Memory Utilization
- Time to Readiness for Managed Resources

The above metrics are very meaningful in terms of provider’s performance. Resource consumption data provides us very 
valuable information for different scenarios/versions/implementations. With the collected data, it is possible to make 
important observations in terms of the effect of a change in the implementation on the resource consumption, trying to 
understand the behavior of the provider under load, the performance impact of the changes made during the versions. The 
information on how long the managed resources are ready is also directly related to the provider’s reconciliation speed. 
Therefore, many interpretations can be made from this data, such as making inferences from the provider’s reconciliation 
behavior, observing the characteristics of different resources, capturing the performance pattern of sync/async resources.

In this context, by developing a tool, our goals are:

- The user can simply observe the performance characteristics of the providers with the collected and reported metrics.
- Providing stable, generic and consistent measurement of performance characteristics of providers for different scenarios.
- Accepting an easily configurable experiment inputs

## Proposal

The objectives and requirements described in the above sections can be achieved through a Go program. We can explain how 
this tool will address them:

**Collecting Performance Metrics:** First of all, we will need data to reveal the characteristics of providers. In this 
context, there are many stable tools that work in the Kubernetes environment. The most widely used of these, Prometheus, 
seems the ideal for data collection. There is a [go-client of the Prometheus]. This client provides an HTTP API of
Prometheus. We can use the prometheus queries for collecting data by using this library. Here are a few possible queries
to use:

- `container_cpu_usage_seconds_total` (by filtering the provider container)
- `container_memory_working_set_bytes` (by filtering the provider container)

**Processing and Reporting Performance Metrics:** Reporting of the collected data can be through an exported document. 
This document contains collected and derived data to facilitate interpretation. For example, if the average CPU utilization 
in a given time period is not provided by Prometheus, this data is reported as a result of preprocessing.

**Providing a Declarative Way to Define the Experiment Details:** In order to be easily identifiable, a method that accepts 
yamls and runs the experiment with this declarative method is applied. In this set of yamls, there may be inputs such as 
resource templates to be deployed to the cluster, configurations of the experiments, configurations for the reporting 
results etc.

- User can provide an example resource manifest that will be deployed to the cluster during experiment.
```yaml
apiVersion: network.azure.jet.crossplane.io/v1alpha2
kind: VirtualNetwork
metadata:
  name: test-vn
spec:
  forProvider:
    addressSpace:
      - 10.0.0.0/16
    dnsServers:
      - 10.0.0.1
      - 10.0.0.2
      - 10.0.0.3
    location: East US
    resourceGroupName: example
  providerConfigRef:
    name: example
```
- User can specify the number of resources that will be created by using the configuration yaml or command-line options.
The developed tool can manipulate the metadata.name for each applying operation.


- User can specify some basic configurations via yaml file.
```yaml
name: test-experiment
provider:
  image: crossplane/provider-jet-azure:v0.8.0
metrics:
  ignore:
  - cpu_utilization
# What is repetition in this context exactly?
count: 1
cluster: Kind | Minikube | Existing
```
- These options can also be provided via command-line options.

**Providing a Comparison Option for the Scenarios/Experiments (Maybe a future work):** Support for comparing results can 
be provided by importing the previous test results. Thus, it can be made easier for the user to interpret the performance 
characteristic changes in a specified time interval. Tool will have a flag `--inputs`, if this flag was passed with values,
tool will work in a comparison mod. The input files for comparison must be the previous outputs of this tool:

```text
Peak CPU Utilization: % 87
Average CPU Utilization: % 66
Peak Memory Utilization: % 28
Average Memory Utilization: % 22
Average Time to Readiness for MRs: 293 seconds
Minimum Time to Readiness for MRs: 144 seconds
Maximum Time to Readiness for MRs: 396 seconds
```

## Future Work

As mentioned above, it is possible to collect many other data to measure the performance of providers. By expanding the 
metric set that the tool will collect and process, it may have a more comprehensive reporting functionality in the future. 
Possible metrics:

- Reconciliation times
- Reconcile error rates
- Request rates
- Wait times of resources in queue
- Response latencies

[go-client of the Prometheus]: https://github.com/prometheus/client_golang
