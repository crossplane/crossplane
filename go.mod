module github.com/crossplaneio/crossplane

go 1.12

require (
	github.com/crossplaneio/crossplane-runtime v0.1.0
	github.com/crossplaneio/crossplane-tools v0.0.0-20191023215726-61fa1eff2a2e
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.3.1
	github.com/onsi/ginkgo v1.9.0 // indirect
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	github.com/spf13/afero v1.2.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apiextensions-apiserver v0.0.0-20190409022649-727a075fdec8
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.0
	sigs.k8s.io/controller-tools v0.2.1
)
