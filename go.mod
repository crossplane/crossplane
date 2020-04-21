module github.com/crossplane/crossplane

go 1.13

replace sigs.k8s.io/controller-runtime => github.com/negz/controller-runtime v0.5.1-0.20200326231846-15460aedd9b7

replace github.com/crossplane/crossplane-runtime => github.com/muvaf/crossplane-runtime v0.0.0-20200421001124-4a7ece2ff53b

require (
	github.com/crossplane/crossplane-runtime v0.7.1-0.20200421211018-be37c50cc2ab
	github.com/crossplane/crossplane-tools v0.0.0-20200219001116-bb8b2ce46330
	github.com/crossplane/oam-kubernetes-runtime v0.0.0-20200422175842-afde24fdf35b
	github.com/docker/distribution v2.7.1+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.4.0
	github.com/google/uuid v1.1.1
	github.com/onsi/gomega v1.8.1
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/pkg/errors v0.8.1
	github.com/spf13/afero v1.2.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.17.3
	k8s.io/apiextensions-apiserver v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v0.17.3
	sigs.k8s.io/controller-runtime v0.5.2
	sigs.k8s.io/controller-tools v0.2.4
)
