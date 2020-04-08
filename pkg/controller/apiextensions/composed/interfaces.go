package composed

import (
	"context"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1/instance"
)

// SpecOps lists the operations that are done on the spec of instance.
type SpecOps interface {
	ResolveSelector(ctx context.Context, cr *instance.InfraInstance) error
}
