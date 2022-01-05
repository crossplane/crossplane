module github.com/crossplane/crossplane

go 1.16

require (
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/Masterminds/semver v1.5.0
	github.com/alecthomas/kong v0.2.17
	github.com/aws/aws-sdk-go v1.31.6 // indirect
	github.com/crossplane/crossplane-runtime v0.15.1-0.20211029211307-c72bcdd922eb
	github.com/google/go-cmp v0.5.6
	github.com/google/go-containerregistry v0.7.0
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20210330174036-3259211c1f24
	github.com/imdario/mergo v0.3.12
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.6.0
	golang.org/x/tools v0.1.5
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/code-generator v0.21.3
	k8s.io/utils v0.0.0-20210722164352-7f3ee0f31471
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/controller-tools v0.6.2
	sigs.k8s.io/yaml v1.2.0
)
