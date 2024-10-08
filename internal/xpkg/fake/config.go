package fake

import (
	"context"

	"github.com/crossplane/crossplane/internal/xpkg"
)

var _ xpkg.ConfigStore = &MockConfigStore{}

// MockConfigStore is a mock ConfigStore.
type MockConfigStore struct {
	MockPullSecretFor func(ctx context.Context, image string) (imageConfig string, pullSecret string, err error)
}

// PullSecretFor calls the underlying MockPullSecretFor.
func (s *MockConfigStore) PullSecretFor(ctx context.Context, image string) (imageConfig string, pullSecret string, err error) {
	return s.MockPullSecretFor(ctx, image)
}

// NewMockConfigStorePullSecretForFn creates a new MockPullSecretFor function for MockConfigStore.
func NewMockConfigStorePullSecretForFn(imageConfig, pullSecret string, err error) func(context.Context, string) (string, string, error) {
	return func(context.Context, string) (string, string, error) {
		return imageConfig, pullSecret, err
	}
}
