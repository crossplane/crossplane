module github.com/crossplane/crossplane

go 1.13

require (
	github.com/alecthomas/kong v0.2.11
	github.com/crossplane/crossplane-runtime v0.9.1-0.20200918014829-e7742464e49b
	github.com/crossplane/oam-kubernetes-runtime v0.0.0-20200426101222-2b61763c2e51
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/google/go-cmp v0.4.0
	github.com/pkg/errors v0.9.1
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
