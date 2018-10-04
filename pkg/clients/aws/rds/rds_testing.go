package rds

import "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"

type MockClient struct {
	MockGetInstance    func(string) (*Instance, error)
	MockCreateInstance func(name, password string, spec *v1alpha1.RDSInstanceSpec) (*Instance, error)
	MockDeleteInstance func(name string) (*Instance, error)
}

// GetInstance finds RDS Instance by name
func (m *MockClient) GetInstance(name string) (*Instance, error) {
	return m.MockGetInstance(name)
}

// CreateInstance creates RDS Instance with provided Specification
func (m *MockClient) CreateInstance(name, password string, spec *v1alpha1.RDSInstanceSpec) (*Instance, error) {
	return m.MockCreateInstance(name, password, spec)
}

// DeleteInstance deletes RDS Instance
func (m *MockClient) DeleteInstance(name string) (*Instance, error) {
	return m.MockDeleteInstance(name)
}
