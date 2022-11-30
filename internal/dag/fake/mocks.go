/*
Copyright 2020 The Crossplane Authors.

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

// Package fake contains mock a Crossplane package DAG.
package fake

import (
	"github.com/crossplane/crossplane/internal/dag"
)

var _ dag.DAG = &MockDag{}

// MockDag is a mock DAG.
type MockDag struct {
	MockInit             func(nodes []dag.Node) ([]dag.Node, error)
	MockAddNode          func(dag.Node) error
	MockAddNodes         func(...dag.Node) error
	MockAddOrUpdateNodes func(...dag.Node)
	MockGetNode          func(identifier string) (dag.Node, error)
	MockAddEdge          func(from string, to dag.Node) (bool, error)
	MockAddEdges         func(edges map[string][]dag.Node) ([]dag.Node, error)
	MockNodeExists       func(identifier string) bool
	MockNodeNeighbors    func(identifier string) ([]dag.Node, error)
	MockTraceNode        func(identifier string) (map[string]dag.Node, error)
	MockSort             func() ([]string, error)
}

// Init calls the underlying MockInit.
func (d *MockDag) Init(nodes []dag.Node) ([]dag.Node, error) {
	return d.MockInit(nodes)
}

// AddNode calls the underlying MockAddNode.
func (d *MockDag) AddNode(n dag.Node) error {
	return d.MockAddNode(n)
}

// AddNodes calls the underlying MockAddNodes.
func (d *MockDag) AddNodes(n ...dag.Node) error {
	return d.MockAddNodes(n...)
}

// AddOrUpdateNodes calls the underlying MockAddOrUpdateNodes.
func (d *MockDag) AddOrUpdateNodes(n ...dag.Node) {
	d.MockAddOrUpdateNodes(n...)
}

// GetNode calls the underlying MockGetNode.
func (d *MockDag) GetNode(i string) (dag.Node, error) {
	return d.MockGetNode(i)
}

// AddEdge calls the underlying MockAddEdge.
func (d *MockDag) AddEdge(from string, to dag.Node) (bool, error) {
	return d.MockAddEdge(from, to)
}

// AddEdges calls the underlying MockAddEdges.
func (d *MockDag) AddEdges(edges map[string][]dag.Node) ([]dag.Node, error) {
	return d.MockAddEdges(edges)
}

// NodeExists calls the underlying MockNodeExists.
func (d *MockDag) NodeExists(i string) bool {
	return d.MockNodeExists(i)
}

// NodeNeighbors calls the underlying MockNodeNeighbors.
func (d *MockDag) NodeNeighbors(i string) ([]dag.Node, error) {
	return d.MockNodeNeighbors(i)
}

// TraceNode calls the underlying MockTraceNode.
func (d *MockDag) TraceNode(i string) (map[string]dag.Node, error) {
	return d.MockTraceNode(i)
}

// Sort calls the underlying MockSort.
func (d *MockDag) Sort() ([]string, error) {
	return d.MockSort()
}
