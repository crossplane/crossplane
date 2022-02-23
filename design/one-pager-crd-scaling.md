# Performance Issues of Deploying Thousands of CRDs
* Owner: Alper Rifat Uluçınar (@ulucinar)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background
With the release of the [Terrajet](https://github.com/crossplane/terrajet) based providers, the Crossplane community has become more aware of some upstream scaling issues related to custom resource definitions. We did some early analysis such as [[1]] and [[2]] to get a better understanding of these issues and as we will discuss in more detail in the “Issues” section, the broader K8s community has already been aware of especially the client-side throttling problems for some time. It’s also not Crossplane alone. [Azure Service Operator](https://github.com/Azure/azure-service-operator), or [GCP Config Connector](https://github.com/GoogleCloudPlatform/k8s-config-connector) are projects that rely on Kubernetes [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) as an extension mechanism and have many CRDs representing associated Cloud resources. 

Kubernetes is a complex ecosystem with many moving parts and we need a deeper understanding for the issues around scaling in the dimension of the total number of CRDs per-cluster. This dimension is not yet officially considered in the scalability thresholds document [[3]] but it will be with good probability. So, as the Crossplane community, we would like to have our use cases considered in relevant contexts, and we would like to gain a good understanding so that we can:
- Establish a common understanding around relevant components and issues
- Discuss and contribute to relevant upstream discussions with the broader community
- Come up with tooling that can help reproducing the issues and assessing the effectiveness of considered solutions
- Establish a common understanding on the Crossplane scenarios and if possible, a set of “definition of done” criterias for the CRD scaling issues, i.e., have a clear expectation on the scenarios we would like to support and on the expected performance in those scenarios.
- …

 

## Issues

We can categorize the issues that we observe when scaling a cluster in the number of installed CRDs in two:
- Client-side issues: Clients of the control-plane (`kubectl`, controllers, etc.) experience extended delays in the requests they make. So far, our observation is that all the issues that we categorize as client-side issues are due to “client-side throttling”, and not caused by the API server itself. These clients can throttle the requests they make to the API server solely as a function of the [rate limiter](https://pkg.go.dev/k8s.io/client-go/util/flowcontrol#RateLimiter) implementation they are using with no feedback from the server. Per our observations, the problematic client-side component that’s responsible for the high number of requests in a unit time (high request rate) is the [discovery client](https://pkg.go.dev/k8s.io/client-go/discovery#DiscoveryInterface). In the [Client-side Throttling](#client-side-throttling) section below, we will examine how `kubectl` behaves when there is a large number of CRDs installed in the cluster and how the discovery cache and the discovery client affect the perceived performance of the `kubectl` commands run. 
- Server-side issues: ...


### Client-side Throttling
`kubectl` maintains a discovery cache for the discovered server-side resources under the (default) filesystem path of `$HOME/.kube/cache/discovery/<host_port>/`. Here, `<host_port>` is a string derived from the API server host and the port number it’s listening on. An example path would be `$HOME/.kube/cache/discovery/exampleaks_8e092dad.hcp.eastus.azmk8s.io_443`, or `$HOME/.kube/cache/discovery/EB788B3B801893B684B4579B2ADF0171.gr7.us_east_1.eks.amazonaws.com`. Under this cache, we have the `servergroups.json` file, which is a JSON-serialized [`v1.APIGroupList`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#APIGroupList) object. Thus, the cache file `$HOME/.kube/cache/discovery/<host_port>/servergroups.json` holds all of the discovered API GroupVersions (GVs) together with their preferred versions from that API service. And for each discovered API GroupVersion, we have a `serverresources.json` that caches metadata about the discovered resources under that GroupVersion, JSON-serialized as a [`v1.APIResourceList`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#APIResourceList). This metadata about resources is crucial for various tasks, such as: 
- Enabling [`meta.RestMapper`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/meta#RESTMapper) implementations to map from GVRs to GVKs, or from GVKs to GVRs, or from partial resource specifications to potential GVRs, etc. 
- Discovery of potential GVRs based on the [short names](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- Determining the scope of a GVR so that the associated resource path can be constructed properly
- Listing resources by category
- …

The discovery client implementation is responsible for populating this cache when:
- The cache does not yet exist on the filesystem (e.g., initial run)
- A cache entry has expired (currently the TTL is hardcoded to be [10 minutes](https://github.com/kubernetes/cli-runtime/blob/09dc8675ba9e1a3291e3f4b7f83a48ecc72e8625/pkg/genericclioptions/config_flags.go#L287))
- The cache is programmatically invalidated

Discovery client starts the discovery process by first [fetching](https://github.com/kubernetes/kubernetes/blob/d5263feb038825197ab426237b111086822366be/staging/src/k8s.io/client-go/discovery/discovery_client.go#L258) the [`v1.APIGroupList`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#APIGroupList) and then [fetching](https://github.com/kubernetes/kubernetes/blob/d5263feb038825197ab426237b111086822366be/staging/src/k8s.io/client-go/discovery/discovery_client.go#L267) the [`v1.APIResourceList`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#APIResourceList) for each GV discovered in the first phase. In this second phase, the discovery client makes [parallel requests](https://github.com/kubernetes/kubernetes/blob/d5263feb038825197ab426237b111086822366be/staging/src/k8s.io/client-go/discovery/discovery_client.go#L363) to the API server to fetch each [`v1.APIResourceList`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#APIResourceList). 

Thus, if the discovery client needs to (re)discover the server-side resources, it has to roughly make at least:
```
2 + <GV count> = 2 + $(kubectl api-versions | wc -l)
```
requests to the API server, assuming no errors are encountered. The initial two HTTP GET requests are the ones to fetch the `v1.APIGroupList`s from the legacy `/api` and `/apis` paths. Also if, for instance, a `kubectl get pods` command is run, there will be a final request to the `/api/v1/namespaces/<namespace>/pods` endpoint to fetch the actual `PodList` resource. As an example, with a `provider-aws@v0.23.0` installation together with some other monitoring and Cloud provider CRDs installed:
```bash
> kubectl api-versions | wc -l
93
```

```bash
> rm -fR  ~/.kube/cache/discovery/<host_port> && k -v=6 get pods 2>&1 | grep "GET" | wc -l
96
```
Please note that 93 of the HTTP GET requests in the above example invocation are done in parallel. 

`kubectl@v1.24` has its discovery client configured with a [`flowcontrol.tokenBucketRateLimiter`](https://github.com/kubernetes/kubernetes/blob/d5263feb038825197ab426237b111086822366be/staging/src/k8s.io/client-go/util/flowcontrol/throttle.go#L53) with a default QPS of 50.0 and default burst of 300.

**NOTE**: As of `kubectl` `v1.23`, the token bucket rate limiter of the discovery client allows bursts of 100 but has a moderately low QPS parameter, the bucket is filled only at 5.0 qps. I believe this is one of the common misunderstandings regarding the behavior of the discovery client `kubectl` uses. [This](https://github.com/kubernetes/kubernetes/blob/39369f1d5432ae97668e39d6e2e5f2c5d60c0340/staging/src/k8s.io/kubectl/pkg/cmd/cmd.go#L297) has been fixed after the `v1.23.0` release, and the fix should be available starting from `v1.24.0`. The [planned release date](https://github.com/kubernetes/sig-release/tree/master/releases/release-1.24) for Kubernetes `v1.24` is **19th April 2022** and the rate limiter parameters fix has not yet been cherry-picked to the other active release branches.

In a `provider-jet-aws@v0.4.0-preview` installation, we have a total of 192 GVs including some other external (to the Crossplane) CRDs. A measurement with the `time` command reveals that it takes ~20.4s for `kubectl get pods` to run with an empty discovery cache. In this scenario, the token bucket rate limiter (b=100, r=5.0 qps) delays one of the discovery client requests for a max amount of ~18.4s, which determines the total delay (because [`APIResource`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#APIResource) discoveries for individual GVs are run in parallel). In this scenario, if we use a token bucket rate limiter with (b=100, r=50.0 qps), then the max throttling delay drops to ~1.8s. This in turn makes the total time spent for a `kubectl get pods` with an empty cache drop to ~10s. It looks like we need an extra ~8s to fully consume the response body in my setup, apart from the client-side throttling. And with the current `v1.24` tbrl(b=300, r=50.0 qps), it takes ~10s to run a `kubectl get pods`. Again, although not explicitly measured, we are waiting for the API server's response body to be fully consumed, similar to the case with tblr(b=100, r=50.0 qps). Token bucket rate limiter's throttling is no longer the bottleneck. Also we no longer observe client-side throttling warning logs in `kubectl@v1.24` output, as expected.

When we have `provider-jet-aws@v0.4.0-preview`, `provider-jet-azure@v0.7.0-preview` and `provider-jet-gcp@v0.2.0-preview` installed in a cluster with some extra CRDs, we have 368 GVs. `kubectl` `v1.23.3` with discovery client tbrl(b=100, r=5.0 qps) takes ~56s to run `kubectl get pods`, where as a current master build of `kubectl` with discovery client tbrl(b=300, r=50.0 qps) takes ~18s to run the same command, a threefold improvement.

**TODO**: Check how much we can scale with the current parameters.And what parameters are suitable for larger # of CRDs.
**TODO**: Brief discussion for the K8s controllers and other clients (Crossplane `kubectl` extension, etc.) context

The increased latencies as exemplified above has the following drawbacks:
- Harms interactive user experience and degrades performance: Cluster admins running `kubectl` commands against their clusters [observe](https://github.com/kubernetes/kubernetes/pull/101634#issuecomment-933851060) these “client-side” throttling messages, even if `kubectl` verbosity level is at 0.
- Has the potential to break shell scripts due to timeouts if the discovery phase is taking longer than anticipated
- Has the potential to break K8s controller reconciliation loops by exceeding reconciliation context deadlines if the initial discovery phase is taking longer than anticipated

Crossplane community and the broader K8s community are aware of these client-side throttling issues and have occasionally reported related issues. Some examples are:
- https://github.com/kubernetes/kubernetes/pull/101634
- https://github.com/kubernetes/kubectl/issues/1126
- https://github.com/crossplane/terrajet/issues/47#issuecomment-916360747
- …


### API Server Resource (CPU/Memory) Usage Increase per CRD

* Showing the increase in a graph picture.
* Why do we care about the increase?
* Profiling data to see what's going on when you register a CRD.
* Incriminating the most impactful threads/routines that cause the increase.

## Criteria Set for Ideal State

In this section, we would like to discuss some Crossplane scenarios that we would like to support and that involve large numbers of CRDs:
1. Installation of a single provider with hundreds of CRDs: As discussed above, installation of a single provider, such as `provider-jet-aws`, results in ~190 GVs being served by the API server. With the current (`v1.23`) set of client-side throttling parameters of `kubectl`, this has adverse effects on the perceived user performance. However, situation improves as the burstiness and the fill rate of the token bucket rate limiter are increased in `kubectl@v1.24`.
2. Installation of multiple providers, such as `provider-jet-{aws,gcp,azure}`, into the same cluster results in ~370 GVs being served by the API server. Even with the updated client-side throttling parameters of `kubectl@v1.24`, client-side performance is severely affected. We also observe issues related to HA in managed control planes with such large numbers of CRDs.

As the Crossplane community, what we would expect in these scenarios are:
- No API service disruptions like the incidents described in [[4]] in cases where we have ~2000 CRDs installed.
- It takes the discovery client no more than 10s to discover all available GVs. This would cover both the initial discovery needed by the controllers and periodic discoveries triggered by other clients such as `kubectl`. We propose a 10s goal because even if client-side throttling is no longer the bottleneck, it looks like that the REST client needs ~8s to fully consume responses (probably another investigation and improvement point).

For the 1st scenario above, we are already in a good position [except](https://github.com/crossplane/crossplane/issues/2895) for GKE zonal and regional clusters as of Kubernetes `v1.23.1`, probably due to the initial control-plane sizing chosen in their control-planes. And regarding the client-side throttling issues, as discussed in the "Cient-side Throttling" section above, with Kubernetes `v1.24` the token bucket rate limiter used by the discovery client allows request bursts upto 300, which is well above a singe provider's (`provider-jet-aws@v0.4.0-preview`) GVs (~200 GVs). So we anticipate an improved UX with the upcoming Kubernetes `v1.24`.

For the 2nd scenario above, we reach ~370 GVs (including some other "external" CRDs) in the cluster, which is above the allowed burstiness of even the `v1.24` discovery client. And again client-side throttling becomes a bottleneck if the discovery client needs to discover the whole API (empty cache, cache invalidation, etc.). Also even on an AKS `v1.22.4` cluster, we were not able to install `provider-jet-{aws,gcp,azure}` `preview` editions together.

When the criterias discussed above are met for the Crossplane scenarios we currently anticipate, and as long as we can satisfy these criterias with future Crossplane or upstream changes, we should be in a good position.


## Testing Against the Criteria Set

One can use the simple Posix `time` utility to measure the time it takes for `kubectl` to just establish the discovery cache:
```bash
rm -fR  ~/.kube/cache/discovery/<host_port> && time (kubectl api-resources > /dev/null)
```

A more realistic example that also performs a final HTTP GET to fetch a `PodList` resource:
```bash
rm -fR  ~/.kube/cache/discovery/<host_port> && time (kubectl get pods > /dev/null)
```

## Action Items
- We can consider cherry-picking [[5]] to all active release branches `v1.23`, `v1.22`, `v1.21`, as the anticipated release date for the `v1.24` release is in April, 2022.
- Open issues regarding API service disruptions for managed control-planes (GKE regional, AKS, EKS, etc.), where we expect high-availability.
- With some insight on server-side issues, we will also need to pursue a set of issues in kube-apiserver and possibly in other control-plane components.
- Initiate further discussions with Kubernetes [sig-scalability](https://github.com/kubernetes/community/tree/master/sig-scalability) community regarding CRD-scalability and bring agreed-upon Crossplane scenarios into their attention.
- ...

## Prior Art
- Some early analysis that revealed OpenAPI spec aggregation as a point for potential improvement: [[1]], [[2]]
- Kubernetes scalability thresholds document: [[3]]
- GA Scale targets for CRDs: https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/95-custom-resource-definitions#scale-targets-for-ga 
- Analysis of high CPU utilization in kube-apiserver after >700 CRDs are installed: https://github.com/crossplane/terrajet/issues/47
- Associated upstream issue: https://github.com/kubernetes/kubernetes/issues/105932 
- Related upstream issue with similar observations but concentrating on high memory consumption during OpenAPI spec aggregation: https://github.com/kubernetes/kubernetes/issues/101755 
- Kermit’s upstream OpenAPI spec lazy-marshalling fix: https://github.com/kubernetes/kube-openapi/pull/251 
- Core crossplane issue where we also considered two workarounds: https://github.com/crossplane/crossplane/issues/2649 
    - A packaging workaround for granularly packaging CRDs instead of putting them into one big provider (especially motivated for the terrajet-based providers)
    - A runtime workaround where packaging is kept intact but a ProviderConfig tells which CRDs in a given provider package are to be installed and their associated controllers be started
- Kubectl discovery client throttling issue (moved to kubectl repo): https://github.com/kubernetes/kubernetes/issues/105489 
- Corresponding kubectl client-side throttling issue in the kubectl repo: https://github.com/kubernetes/kubectl/issues/1126 
    - Closed by https://github.com/kubernetes/kubernetes/pull/105520 ineffectively bumping discovery burst to 300 
- PR for fixing the burstiness parameter bump: [[5]] 
- A blogpost on client-side throttling from that fix’s author: https://jonnylangefeld.com/blog/the-kubernetes-discovery-cache-blessing-and-curse 
- An issue for making the discovery cache TTL configurable (currently hard-coded at 10 min): https://github.com/kubernetes/kubernetes/issues/107130 
- PR that adds “cache-ttl” flag to make the TTL configurable: https://github.com/kubernetes/kubernetes/pull/107141 
- Nic’s proposal (PR) to disable client-side throttling and leave rate-limiting to AFP: https://github.com/kubernetes/kubernetes/pull/106016 
- Nic’s message about his rate-limit disabling PR in slack: https://kubernetes.slack.com/archives/C0EG7JC6T/p1636491191187300 
- Discussion on the server-side issues on slack that Nic initiated: https://kubernetes.slack.com/archives/C0EG7JC6T/p1634838218016300 



[1]: https://github.com/crossplane/terrajet/issues/47#issuecomment-920316141
[2]: https://github.com/crossplane/terrajet/issues/47#issuecomment-920441882
[3]: https://github.com/kubernetes/community/blob/master/sig-scalability/configs-and-limits/thresholds.md
[4]: https://github.com/crossplane/crossplane/issues/2895
[5]: https://github.com/kubernetes/kubernetes/pull/107131
