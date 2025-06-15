package fake

import (
	"context"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

var _ xpkg.ConfigStore = &MockConfigStore{}

// MockConfigStore is a mock ConfigStore.
type MockConfigStore struct {
	MockPullSecretFor              func(ctx context.Context, image string) (imageConfig string, pullSecret string, err error)
	MockImageVerificationConfigFor func(ctx context.Context, image string) (imageConfig string, verificationConfig *v1beta1.ImageVerification, err error)
	MockRewritePath                func(ctx context.Context, image string) (imageConfig, newPath string, err error)
}

// PullSecretFor calls the underlying MockPullSecretFor.
func (s *MockConfigStore) PullSecretFor(ctx context.Context, image string) (imageConfig string, pullSecret string, err error) {
	return s.MockPullSecretFor(ctx, image)
}

// ImageVerificationConfigFor calls the underlying MockImageVerificationConfigFor.
func (s *MockConfigStore) ImageVerificationConfigFor(ctx context.Context, image string) (imageConfig string, verificationConfig *v1beta1.ImageVerification, err error) {
	return s.MockImageVerificationConfigFor(ctx, image)
}

// RewritePath calls the underlying MockRewritePath.
func (s *MockConfigStore) RewritePath(ctx context.Context, image string) (imageConfig, newPath string, err error) {
	return s.MockRewritePath(ctx, image)
}

// NewMockConfigStorePullSecretForFn creates a new MockPullSecretFor function for MockConfigStore.
func NewMockConfigStorePullSecretForFn(imageConfig, pullSecret string, err error) func(context.Context, string) (string, string, error) {
	return func(context.Context, string) (string, string, error) {
		return imageConfig, pullSecret, err
	}
}

// NewMockConfigStoreImageVerificationConfigForFn creates a new MockImageVerificationConfigFor function for MockConfigStore.
func NewMockConfigStoreImageVerificationConfigForFn(imageConfig string, verificationConfig *v1beta1.ImageVerification, err error) func(context.Context, string) (string, *v1beta1.ImageVerification, error) {
	return func(context.Context, string) (string, *v1beta1.ImageVerification, error) {
		return imageConfig, verificationConfig, err
	}
}

// NewMockRewritePathFn creates a new MockRewritePath function for
// MockConfigStore.
func NewMockRewritePathFn(imageConfig, newPath string, err error) func(context.Context, string) (string, string, error) {
	return func(_ context.Context, _ string) (string, string, error) {
		return imageConfig, newPath, err
	}
}
