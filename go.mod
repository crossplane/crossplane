module github.com/crossplane/crossplane

go 1.13

replace github.com/crossplane/crossplane-runtime => github.com/muvaf/crossplane-runtime v0.0.0-20200910111027-446c12e8ee41

require (
	github.com/alecthomas/kong v0.2.11
	github.com/crossplane/crossplane-runtime v0.9.1-0.20200909225216-9b321c2bc8e6
	github.com/crossplane/crossplane-tools v0.0.0-20200412230150-efd0edd4565b
	github.com/crossplane/oam-kubernetes-runtime v0.0.0-20200426101222-2b61763c2e51
	github.com/docker/distribution v2.7.1+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.4.0
	github.com/google/uuid v1.1.1
	github.com/onsi/gomega v1.10.1
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.2.2
	golang.org/x/net v0.0.0-20200520004742-59133d7f0dd7
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	k8s.io/code-generator v0.18.6
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.2.4
)
