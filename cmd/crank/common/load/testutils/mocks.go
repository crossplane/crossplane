// Package testutils is for test utilities.
package testutils

import un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// MockLoader represents a mock Loader for testing.
type MockLoader struct {
	Resources []*un.Unstructured
	Err       error
}

// Load returns the resources and/or error on this mock.
func (m *MockLoader) Load() ([]*un.Unstructured, error) {
	return m.Resources, m.Err
}
