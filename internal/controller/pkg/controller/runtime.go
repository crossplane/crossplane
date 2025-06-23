package controller

// ActiveRuntime is the runtime to use for packages with runtime.
type ActiveRuntime struct {
	runtimes       map[string]PackageRuntime
	defaultRuntime PackageRuntime
}

// RuntimeOption defines options to build up an active runtime.
type RuntimeOption func(runtime *ActiveRuntime)

// WithDefaultPackageRuntime marks a default runtime for unset kinds.
func WithDefaultPackageRuntime(runtime PackageRuntime) RuntimeOption {
	return func(ar *ActiveRuntime) {
		ar.defaultRuntime = runtime
	}
}

// WithPackageRuntime associates a runtime to a kind.
func WithPackageRuntime(kind string, runtime PackageRuntime) RuntimeOption {
	return func(ar *ActiveRuntime) {
		ar.runtimes[kind] = runtime
	}
}

// NewActiveRuntime builds an ActiveRuntime based on the provided options.
func NewActiveRuntime(o ...RuntimeOption) ActiveRuntime {
	r := ActiveRuntime{
		runtimes: make(map[string]PackageRuntime),
	}
	for _, o := range o {
		o(&r)
	}
	if r.defaultRuntime == PackageRuntimeUnspecified {
		r.defaultRuntime = PackageRuntimeDeployment
	}
	return r
}

// For returns the associated runtime for a given kind.
func (r ActiveRuntime) For(kind string) PackageRuntime {
	if runtime, ok := r.runtimes[kind]; ok {
		return runtime
	}
	return r.defaultRuntime
}

// PackageRuntime is the runtime to use for packages with runtime.
type PackageRuntime string

const (
	// PackageRuntimeUnspecified means no package runtime is specified.
	PackageRuntimeUnspecified PackageRuntime = ""

	// PackageRuntimeDeployment uses a Kubernetes Deployment as the package
	// runtime.
	PackageRuntimeDeployment PackageRuntime = "Deployment"

	// PackageRuntimeExternal defer package runtime to an external controller.
	PackageRuntimeExternal PackageRuntime = "External"

	// PackageRuntimeIndependent allow per package kind runtime selection.
	PackageRuntimeIndependent PackageRuntime = "Independent"
)
