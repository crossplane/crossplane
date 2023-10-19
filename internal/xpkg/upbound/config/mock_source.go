/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

// MockSource is a mock source.
type MockSource struct {
	InitializeFn   func() error
	GetConfigFn    func() (*Config, error)
	UpdateConfigFn func(*Config) error
}

// Initialize calls the underlying initialize function.
func (m *MockSource) Initialize() error {
	return m.InitializeFn()
}

// GetConfig calls the underlying get config function.
func (m *MockSource) GetConfig() (*Config, error) {
	return m.GetConfigFn()
}

// UpdateConfig calls the underlying update config function.
func (m *MockSource) UpdateConfig(c *Config) error {
	return m.UpdateConfigFn(c)
}
