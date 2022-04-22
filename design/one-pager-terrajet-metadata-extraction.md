# Metadata Extraction from Terraform Registry for Terrajet-based providers
* Owner: Alper Rifat Uluçınar (@ulucinar)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background
For providers generated using [Terrajet], the number of managed resources can
exceed [several hundreds](provider-jet-aws-preview), and especially for the big
three Terrajet-based providers ([provider-jet-aws], [provider-jet-gcp] and
[provider-jet-azure]), it's very inconvenient and time consuming to manually
author example manifests for all those generated resources. The convention we
have adopted so far is to manually add example manifests for the resources we
explicity configure in their respective pull requests. 

Another dimension we need to consider is that currently we are lacking API
documentation for the generated resources. Although it's possible to redirect
users of those APIs to the [Terraform registry], it's desirable to have the
documentation generated together with the API (as comments on the associated
`struct`s and fields), and have them published on `doc.crds.dev`.

There is also a wealth of metadata that we can use to enrich the Terrajet-based
providers and the generated resources such as category names for the Terraform
resources. For example, a provider implementation may opt to use the category
names to group respective APIs. Or the examples provided in the Terraform
registry hint at reference fields that can help us in auto-generating
cross-resource references (that appear in those HCL configurations).

While working on generating example manifests for the big three Terrajet-based
providers `provider-jet-aws`, `provider-jet-gcp` and `provider-jet-azure` in the
context of the corresponding [Terrajet issue #48], we have seen utility in
extracting such metadata from the Terraform registry and use it to generate
example manifests and documentation. In this document, we would like to propose:
- A metadata format that we can optionally use in the Terrajet-based provider
repositories to generate example manifests, documentation, etc., 
- A concept of metadata extractors from Terraform registry and potentially from
  other sources for Terrajet-based providers, 
- A new Terrajet codegen pipeline to generate example manifests, which can
optionally be invoked in Terrajet-based providers during code generation,
accepting the scraped metadata from the Terraform registry. 
- Extension of existing Terrajet codegen pipelines to also generate
  documentation on `struct`s and fields.

## Goals
We would like to achieve the following goals with this proposal:
- Terrajet [resource configuration][resource configuration API] framework allows
  us to customize/adjust the code generation pipeline invoked for generating the
  Custom Resource Definitions (CRDs) generated for the terrajet-based provider's
  managed resources. And as these configuration overrides evolve, we would like
  to have the artifacts produced be always consistent with the most recent
  configuration. For instance, when a managed resource's kind or API group
  change, or when its version is bumped, such changes should appropriately be
  reflected in any artifacts (such as the provided example manifests)
  automatically. Also we should be generating any relevant artifacts for a newly
  added managed resource (example manifests, documentation, etc.) automatically. 
- The proposed new pipeline(s) or extensions of the existing code generation
  pipelines must be optional. If, for example, an example generation pipeline is
  not configured in a Terrajet-based provider repo, or if the already existing
  code generation pipeline is not configured to also generate documentation,
  then the behavior of the configured pipelines should not change. Thus,
  configuration of the new pipelines or enhancement of existing ones with
  registry-scraped metada should be optional.
- Like existing Terrajet pipelines, newly added registry metadata based
  pipelines should be stable, i.e., running them on the same metadata must
  always produce the same output. Simiarly, any extension of the existing
  pipelines with registry metadata must preserve their stability.
- We would like to have means of correcting/adjusting scraped metadata before
  it's input to the codegen pipelines. This would allow us to make manual
  corrections/enhancements on the output of a scraper, or even manually craft
  complete or semi-complete registry metadata documents, if for example the
  provider is small (in the number of resources it supports), and an automatic
  scraper is not immediately available. This will also allow us, if needed, to
  have different scrapers that produce output in the same metadata format. For
  instance, we may have a relatively complex scraper for extracting metadata
  from the Terraform registry, and another relatively simple one that just adds
  example HCL configurations by reading them from their respective
  [files][aws-example-configurations]. This will allow different scraper
  implementations to be able to fetch metadata from different sources but the
  Terrajet pipelines will always be working on a well defined format regardless
  of how those metadata are scraped.


## Proposal
We propose to scrape metadata about the Terraform providers and their resources
from the [Terraform registry] and to define an easily consumable format for
storing the scraped metadata. We can use the extracted metadata later in the
various automated pipelines (in the existing Terrajet code-generation pipeline, and
further in the newly added pipelines, such as an example-manifest generation
pipeline). We can scrape the example Terraform configurations represented in
[HCL], store them in the common metadata format, and then with a new
example-manifest generation pipeline run right after the existing Terrajet
code-generation pipeline, we can generate the YAML example manifests for the
generated managed resources. Or, we can incorporate Terraform resource
documentation, which is extracted with these scrapers from the Terraform registry and
stored in the common metadata format, in the existing Terrajet code-generation
pipeline to generate the CRD documentation (embedded as Go comments in the CRD
`struct` types and their corresponding fields). There are also other use cases
that we can enable using this extracted registry metadata, such as standardizing
API groups of the generated managed resources.

There is a plethora of metadata that we can extract from the Terraform registry.
HCL-formatted example resource configurations in the registry can be used for
generating YAML-formatted managed resource example manifests (this has already
been accomplished in [[1]]). Documentation of the Terraform resources can be
incorporated as the CRD documentation for the generated Crossplane managed
resources. Subcategory information or path components extracted from Terraform
import statements are candidates to be used as the API group names of the
generated managed resources. 

In the following sections, we describe the proposed [common metadata
format][Proposed Metadata Format] for storing the extracted registry metadata.
Then we discuss how we can [scrape metadata][Metadata scrapers] from the
registry and how we can incorporate this metadata in new and existing
[code-generation pipelines][Terrajet Codegen Pipelines Consuming Metadata]. We
conclude with a discussion on some [alternatives considered][Alternatives
Considered] and [future considerations][Future Considerations]. 

### Proposed Metadata Format
The proposed syntax for scraped metadata documents is YAML as we would also like
the metadata to be human readable, searchable and maintainable if needed, as
well as it will be input to a number of code-generation pipelines. As
maintenance tasks on the scraped metadata, we can consider correcting typos in
the scraped documentation and improving example configurations, and the similar.
A concrete example of a scraped registry metadata document for a resource named
`azurerm_analysis_services_server` of the native Terraform provider
[terraform-provider-azurerm] could be as follows. Please note that this
represents a full instantiation of the proposed format with detailed comments
explaining the keys and with example use cases for the extracted metadata.

```yaml
# a Terraform native resource name defined in the (native) Terraform provider
name: azurerm_analysis_services_server
# sub-category metadata for the resource extracted from Terraform registry, if available. 
# Candidate to be used as API group names in the generated Terrajet provider, if desired.
subCategory: Analysis Services
# description for the resource extracted from Terraform registry, if available. 
# Candidate to be used as the CRD type documentation
description: Manages an Analysis Services Server.
# title for the resource as it appears in the registry.
titleName: azurerm_analysis_services_server
# Array of example HCL configurations available for the Terraform resource. 
# Terraform registry contains examples but there can be other sources as well.
examples:
    # example configuration in HCL syntax
    - manifest: |-
        {
          "admin_users": [
            "myuser@domain.tld"
          ],
          "enable_power_bi_service": true,
          "ipv4_firewall_rule": [
            {
              "name": "myRule1",
              "range_end": "210.117.252.255",
              "range_start": "210.117.252.0"
            }
          ],
          "location": "northeurope",
          "name": "analysisservicesserver",
          "resource_group_name": "${azurerm_resource_group.rg.name}",
          "sku": "S0",
          "tags": {
            "abc": 123
          }
        }
      # reference parameters extracted from Terraform registry examples
      # map from referer parameter names to referee <target resource type>.<target field>
      references:
        # for example, "azurerm_analysis_services_server" has a parameter 
        # named "resource_group_name" that refers to a "azurerm_resource_group"'s 
        # "name" parameter
        # Candidate for auto-generating cross-resource references
        resource_group_name: azurerm_resource_group.name
# scraped Terraform registry docs for the parameters and attributes of the resource
argumentDocs:
    # parameters with non-block values map directly to doc strings
    admin_users: '- (Optional) List of email addresses of admin users.'
    backup_blob_container_uri: '- (Optional) URI and SAS token for a blob container to store backups.'
    enable_power_bi_service: '- (Optional) Indicates if the Power BI service is allowed to access or not.'
    # exported attributes appear under the "exportedAttributes" map (as a block)
    exportedAttributes:
        id: '- The ID of the Analysis Services Server.'
        server_full_name: '- The full name of the Analysis Services Server.'            
    # parameters with block values are maps
    ipv4_firewall_rule:
        name: '- (Required) Specifies the name of the firewall rule.'
        # if the block-valued parameter has itself a description, it appears under "nodeText"
        # We assume "nodeText" is not a valid parameter/attribute name
        nodeText: '- (Optional) One or more ipv4_firewall_rule block(s) as defined below.'
        range_end: '- (Required) End of the firewall rule range as IPv4 address.'
        range_start: '- (Required) Start of the firewall rule range as IPv4 address.'
    location: '- (Required) The Azure location where the Analysis Services Server exists. Changing this forces a new resource to be created.'
    name: '- (Required) The name of the Analysis Services Server. Changing this forces a new resource to be created.'
    querypool_connection_mode: '- (Optional) Controls how the read-write server is used in the query pool. If this value is set to All then read-write servers are also used for queries. Otherwise with ReadOnly these servers do not participate in query operations.'
    resource_group_name: '- (Required) The name of the Resource Group in which the Analysis Services Server should be exist. Changing this forces a new resource to be created.'
    sku: '- (Required) SKU for the Analysis Services Server. Possible values are: D1, B1, B2, S0, S1, S2, S4, S8, S9, S8v2 and S9v2.'
    timeouts:
        create: '- (Defaults to 30 minutes) Used when creating the Analysis Services Server.'
        delete: '- (Defaults to 30 minutes) Used when deleting the Analysis Services Server.'
        read: '- (Defaults to 5 minutes) Used when retrieving the Analysis Services Server.'
        update: '- (Defaults to 30 minutes) Used when updating the Analysis Services Server.'
# import statement scraped from the Terraform registry, if available
# Can be used for advanced purposes, such as constructing resource config "ExternalName.GetIDFn" functions, etc.
importStatements:
    - terraform import azurerm_analysis_services_server.server /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/resourcegroup1/providers/Microsoft.AnalysisServices/servers/server1
```

Each Terraform resource's scraped metadata is stored in its own YAML-formatted
file. These resource specific files could each be named as `<Terraform resource
type>.yaml`, e.g., `azurerm_analysis_services_server.yaml`.


Another alternative for storing scraped documentation could be to have qualified
names under the `argumentDocs` with a flat hierarchy, e.g., instead of a nested
`ipv4_firewall_rule` block represented as a map, we could have its block
parameters qualified with the configuration block name
(`ipv4_firewall_rule.range_start`, `ipv4_firewall_rule.range_end`, etc.) Then
`argumentDocs` would become a simple `map[string]string`. The structure proposed
above results in shorter key names and is an object-like representation
capturing the hierarchy between nested document elements.

This proposal opts to have per-resource YAML metadata files.
Instead of a `resources` map in a single YAML file under which all provider
resources' metadata are stored under their own keys, we have each resource's metadata
stored in their own resource specific YAML-formatted files. The alternative
would look like something similar to:

```yaml
# Terraform native provider name
name: hashicorp/terraform-provider-azurerm
# map from Terraform native resource names to scraped resource metadata
resources:
    # a Terraform native resource name defined in the provider
    azurerm_analysis_services_server:
      # Rest is the same as the above document but azurerm_analysis_services_server's scraped metadata residing under its key
      # sub-category metadata for the resource extracted from Terraform registry, if available. 
      # Candidate to be used as API group names in the generated Terrajet provider, if desired.
      subCategory: Analysis Services
      ...
    # another Terraform native resource defined in the provider. Its metadata is stored under its own key.
    azurerm_resourcegroup:
      ...
```

We opt for a per-resource metadata file approach because:
- For large providers, such as `provider-jet-aws`, a monolithic metadata file
  would quickly become large, and it would be more difficult to manually edit it
  (please refer to [Proposed Metadata Format] section for some prospective
  maintenance tasks). 
- As we will discuss in the [Future Considerations] section, we can consider
  maintaining the manual modifications to the scraped metadata not in the
  metadata file itself but as separate patch documents (in tandem with a tool
  like [Kustomize]). Preparing patches for more modular units will be easier if
  we want to do something similar to this.


### Metadata scrapers
Although not validated on all of available Terraform providers, at least, the
big three Terraform providers ([terraform-provider-aws],
[terraform-provider-azurerm] and [terraform-provider-google]) all have Terraform
registry content in their respective repositories and use markdown documents
with a common structure. Our assumption is that Terraform registry website is
also generated using these markdown files:
- For `terraform-provider-aws`:
  https://github.com/hashicorp/terraform-provider-aws/tree/main/website/docs/r
- For `terraform-provider-azurerm`:
  https://github.com/hashicorp/terraform-provider-azurerm/tree/main/website/docs/r
- For `terraform-provider-google`:
  https://github.com/hashicorp/terraform-provider-google/tree/main/website/docs/r

Thus a common metadata scraper implementation can extract metadata from these
well-formatted per-resource markdown documents. Any spotted errors can then
potentially be corrected manually in the scraped YAML metadata document.
Scrapers can optionally be chained: If desired, another scraper can append
example HCL configurations read from a different source (such as the `examples`
folder found in some of the Terraform native provider repositories as discussed
above). 

For a specific version of the native Terraform provider consumed. i.e., against
which the terrajet-based Crossplane provider's managed resources are generated,
we can have the scrapers run once, via a new `Makefile` target. The result is a
snapshot of the Terraform registry metadata for a specific version of the native
provider. And as long as we do not bump the native provider version consumed, we
do *not* need to run the scrapers again, e.g. as part of the `make generate`
target. However, when the native provider version is bumped, it makes sense to
rerun the scrapers to capture new metadata, for instance, for the newly added
Terraform resources in the newer version of the native provider. The resulting
artifacts (a set of per-resource metadata files) would then be committed to the
Crossplane provider repository to be used in the code generation pipelines that
we will discuss in the following sections. 

If there are any manual modifications made in the scraped metadata files,
rerunning the scrapers would get them lost. As discussed above, rerunning the
scrapers without a native provider version bump is not expected to be common.
However, we may also want to carry certain metadata modifications (overrides) to
the new versions of these files after an actual version bump. In order to make
this process automatic and easier to maintain, as discussed in the [Future
Considerations] section, we can consider maintaining such modifications as
separate patch files.

As discussed above, we would like to have these scrapers run as needed, produce
their output in the proposed common metadata format, and to have the metadata
documents added to their respective repositories. However, we can then have the
corresponding pipelines run each time with a `make generate`, just like the
existing codegen pipelines we have. This would allow us to separate the
lifecycles of metadata-scraping and code generation.

For most Terraform native providers, we anticipate that Terraform registry
scrapers will **not** run on HTTP, as the resource markdown files are part of
their corresponding provider repositories. They can just read those markdowns
from a pointed directory in the local filesystem, which is specified as a
command-line argument, for instance. The new `Makefile` target added to run
these scrapers can first fetch the relevant paths containing the markdown files
with `git` and run the scrapers on them. This will be easier and more efficient
than requesting each markdown file via `HTTP` because:
- We will not need to parse a separate index file to learn which documents to
  fetch.
- We will not make separate requests for each markdown document, the `git`
  tooling should be capable of bundling/compressing these sets of files (under a
  common repository path) itself.

However, the scraper is independent of the transport being employed.

As already indicated, if it turns out that a common registry scraper
implementation is not suitable for a specific Terraform native provider, then a
new scraper can be written as long as it produces metadata output in the
expected metadata format by the Terrajet pipelines. Or even, if the cost of
writing a new scraper is higher than manually authoring the metadata YAML
documents (e.g., the number of resources in the native provider is small), we
can just prepare the metadata YAML(s) by hand, just like the Terraform community
manually maintains the corresponding markdown documents for the Terraform
registry.

### Terrajet Codegen Pipelines Consuming Metadata
As [implemented][terrajet-pr-173] in the context of the [Terrajet issue #48] for
example manifest generation for the big three providers, we could have some
configurable codegen pipelines that consume the YAML resource metadata file(s)
and produce example manifests, CRD documentation, etc. (as discussed above). The
configured pipelines should not fail if the necessary metadata is missing: For
instance, the [example manifest generation pipeline] should simply not generate
an example manifest for a managed resource, if no sample HCL configuration is
available for the corresponding Terraform resource in the metadata. Or, the
metadata-enhanced CRD generation pipeline should simply skip doc comments if
none or some are not available in the corresponding metadata document. 

Metadata is valuable; the scrapers should capture as much metadata as possible
and store them in the common format, even for future use cases we do not yet
envision. New Terrajet pipelines can be added, or existing ones can be enhanced
to support advanced use cases. One such proposal could be to extend the CRD
generation pipeline to employ the `resource.subCategory` metadata to determine
the API group of a generated CRD (after some simple string processing). Or as an
alternative, another provider could use the `resource.importStatements` metadata
for exactly the same purpose. For example, `provider-jet-azure` currently
[uses][provider-jet-azure-group-config] what we call as the Microsoft provider
name as a default for the API groups of generated resources. Of course, resource
specific manual overrides are always possible via the [resource configuration
API] and for `provider-jet-azure`, most resource IDs have the Microsoft provider
name as a component such as:
```
/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/resourcegroup1/providers/Microsoft.AnalysisServices/servers/server1
```
Because these ID strings appear in the import statements of most
`terraform-provider-azurerm` resources, using such metadata enables us to have a
consistent, repo-wide defaulting for the API group names of the generated resources. 


### Alternatives Considered

### Future Considerations

[1]: https://github.com/crossplane/terrajet/issues/48
[Terrajet]: https://github.com/crossplane/terrajet
[provider-jet-aws-preview]:
    https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-aws@v0.4.0-preview
[Terraform registry]: https://registry.terraform.io/
[provider-jet-aws]: https://github.com/crossplane-contrib/provider-jet-aws
[provider-jet-gcp]: https://github.com/crossplane-contrib/provider-jet-gcp
[provider-jet-azure]: https://github.com/crossplane-contrib/provider-jet-azure
[Terrajet issue #48]: https://github.com/crossplane/terrajet/issues/48
[aws-example-configurations]:
    https://github.com/hashicorp/terraform-provider-aws/tree/main/examples
[terraform-provider-azurerm]:
    https://github.com/hashicorp/terraform-provider-azurerm
[terraform-provider-azurerm]:
    https://github.com/hashicorp/terraform-provider-azurerm
[terraform-provider-aws]: https://github.com/hashicorp/terraform-provider-aws
[terraform-provider-google]:
    https://github.com/hashicorp/terraform-provider-google
[terrajet-pr-173]: https://github.com/crossplane/terrajet/pull/173
[example manifest generation pipeline]:
    https://github.com/ulucinar/terrajet/blob/fix-48/pkg/pipeline/example.go
[provider-jet-azure-group-config]:
    https://github.com/crossplane-contrib/provider-jet-azure/blob/main/config/apigroup_config.go
[resource configuration API]:
    https://github.com/crossplane/terrajet/blob/main/pkg/config/resource.go
[HCL]: https://github.com/hashicorp/hcl
[Kustomize]: https://kustomize.io/