module github.com/crossplane/crossplane

go 1.13

require (
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/Masterminds/semver v1.5.0
	github.com/alecthomas/kong v0.2.11
	github.com/aws/aws-sdk-go v1.31.6 // indirect
	github.com/crossplane/crossplane-runtime v0.14.0
	github.com/docker/cli v0.0.0-20200915230204-cd8016b6bcc5 // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20200926000217-2617742802f6+incompatible // indirect
	github.com/google/go-cmp v0.5.2
	github.com/google/go-containerregistry v0.4.1
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20210330174036-3259211c1f24
	github.com/imdario/mergo v0.3.11
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.4.1
	k8s.io/api v0.20.1
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v0.20.1
	k8s.io/code-generator v0.20.1
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.8.0
	sigs.k8s.io/controller-tools v0.3.0
	sigs.k8s.io/yaml v1.2.0
)
