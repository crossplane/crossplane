module github.com/crossplaneio/crossplane

go 1.13

require (
	github.com/crossplaneio/crossplane-runtime v0.3.1-0.20200114203641-5ece4af54b32
	github.com/crossplaneio/crossplane-tools v0.0.0-20191023215726-61fa1eff2a2e
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.3.1
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.8.1
	github.com/spf13/afero v1.2.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.17.0
	k8s.io/apiextensions-apiserver v0.0.0-20190918161926-8f644eb6e783
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.17.0
	k8s.io/kubectl v0.17.0
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/controller-tools v0.2.4
)
