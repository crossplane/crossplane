---  
title: Self-Signed CA Certs 
toc: true  
weight: 270  
indent: true  
---  

# Overview of Crossplane for Registry with Self-Signed CA Certificate  

> ! Using self-signed certificates is not advised in production, it is 
recommended to only use self-signed certificates for testing.

When Crossplane loads Configuration and Provider Packages from private 
registries, it must be configured to trust the CA and Intermediate certs. 

Crossplane needs to be installed via the Helm chart with the 
`registryCaBundleConfig.name` and `registryCaBundleConfig.key` parameters 
defined. See [Install Crossplane].

## Conifgure

1. Create a CA Bundle (A file containing your Root and Intermediate 
certificates in a specific order). This can be done with any text editor or 
from the command line, so long as the resulting file contains all required crt 
files in the proper order. In many cases, this will be either a single 
self-signed Root CA crt file, or an Intermediate crt and Root crt file. The 
order of the crt files should be from lowest to highest in signing order. 
For example, if you have a chain of two certificates below your Root 
certificate, you place the bottom level Intermediate cert at the beginning of 
the file, then the Intermediate cert that singed that cert, then the Root cert 
that signed that cert.

2. Save the files as `[yourdomain].ca-bundle`.

3. Create a Kubernetes ConfigMap in your Crossplane system namespace:

```
kubectl -n [Crossplane system namespace] create cm ca-bundle-config \
--from-file=ca-bundle=./[yourdomain].ca-bundle
```

4. Set the `registryCaBundleConfig.name` Helm chart parameter to 
`ca-bundle-config` and the `registryCaBundleConfig.key` parameter to 
`ca-bundle`.

> Providing Helm with parameter values is convered in the Helm docs, 
[Helm install](https://helm.sh/docs/helm/helm_install/). An example block  
in an `override.yaml` file would look like this:
```
  registryCaBundleConfig:
    name: ca-bundle-config
    key: ca-bundle
```


[Install Crossplane]: ../reference/install.md