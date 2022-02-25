---  
title: Fetch Packages from Registry with Self-Signed CA Certificate  
toc: true  
weight: 270  
indent: true  
---  

# Overview of Crossplane for Registry with Self-Signed CA Certificate  

Crossplane uses the go-containerregistry remote pkg as the client to fetch
package manifests and images from OCI registries. It uses the 
http.DefaultTransport, meaning that it loads the default system certificates.
This precludes fetching packages from private registries that may use certs
from a self-signed CA.

The default transport can be overridden with one that passes additional certs
to the TLSConfig. This allows for self-signed certificate authorities to be
trusted.

Crossplane needs to be installed via the Helm chart with the 
`registryCaBundleConfig.name` and `registryCaBundleConfig.key` parameters 
defined. See [Install Crossplane](https://crossplane.io/docs/v1.6/reference/install.html#uninstalling-the-chart).

## Conifgure

1. Create a CA Bundle (A file containing your Root and Intermediate 
certificates in a specific order). This can be done with any text editor or 
from the command line, so long as the resulting file contains all required crt 
files in the proper order. In many cases, this will be either a single 
self-signed Root CA crt file, or an Intermediate crt and Root crt file.The 
order of the crt files should be from lowest to highest in signing order. 
For example, if you have a chain of two certificaes below your Root 
certificate, you place the bottom level Intermeidate cert at the beginning of 
the file, then the Intermediate cert that singed that cert, then the Root cert 
that signed that cert.

2. Save the files as [yourdomain].ca-bundle.

3. Create a Kubernetes ConfigMap in your Crossplane system namespace:

```
kubectl -n [Crossplane system namespace] create cm ca-bundle-config /
--from-file=ca-bundle=./[yourdomain].ca-bundle
```

4. Set the `registryCaBundleConfig.name` Helm chart paramater to 
`ca-bundle-config` and the `registryCaBundleConfig.key` parameter to 
`ca-bundle`.


