/*
Copyright 2024 The Crossplane Authors.

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

// Package dag implements a Directed Acyclic Graph for Package dependencies.
package dag

import (
	"github.com/Masterminds/semver"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// MapUpgradingDag is a directed acyclic graph implementation that uses a map for its
// underlying data structure and has the ability to distinguish upgradable nodes.
type MapUpgradingDag struct {
	nodes map[string]Node
}

// NewUpgradingDAGFn is a function that returns a DAG.
type NewUpgradingDAGFn func() DAG

// NewUpgradingMapDag creates a new MapDag.
func NewUpgradingMapDag() DAG {
	return &MapUpgradingDag{nodes: map[string]Node{}}
}

// Init initializes a MapDag and implies missing destination nodes. Any implied
// nodes are returned. Any existing nodes are cleared.
func (d *MapUpgradingDag) Init(nodes []Node) ([]Node, error) {
	d.nodes = map[string]Node{}
	// Add all nodes before adding edges so we know what nodes were implied.
	for _, node := range nodes {
		if err := d.AddNode(node); err != nil {
			return nil, err
		}
	}
	var implied []Node
	for _, node := range nodes {
		miss, err := d.AddEdges(map[string][]Node{
			node.Identifier(): node.Neighbors(),
		})
		if err != nil {
			return nil, err
		}
		implied = append(implied, miss...)
	}

	return implied, nil
}

// AddNodes adds nodes to the graph.
func (d *MapUpgradingDag) AddNodes(nodes ...Node) error {
	for _, n := range nodes {
		if err := d.AddNode(n); err != nil {
			return err
		}
	}
	return nil
}

// AddNode adds a node to the graph.
func (d *MapUpgradingDag) AddNode(node Node) error {
	if _, ok := d.nodes[node.Identifier()]; ok {
		return errors.Errorf("node %s already exists", node.Identifier())
	}
	d.nodes[node.Identifier()] = node
	return nil
}

// AddOrUpdateNodes adds new nodes or updates the existing ones with the same
// identifier.
func (d *MapUpgradingDag) AddOrUpdateNodes(nodes ...Node) {
	for _, node := range nodes {
		if _, ok := d.nodes[node.Identifier()]; ok {
			node.AddParentConstraints(d.nodes[node.Identifier()].GetParentConstraints())
		}
		d.nodes[node.Identifier()] = node
	}
}

// NodeExists checks whether a node exists.
func (d *MapUpgradingDag) NodeExists(identifier string) bool {
	_, exists := d.nodes[identifier]
	return exists
}

// NodeNeighbors returns a node's neighbors.
func (d *MapUpgradingDag) NodeNeighbors(identifier string) ([]Node, error) {
	if _, ok := d.nodes[identifier]; !ok {
		return nil, errors.Errorf("node %s does not exist", identifier)
	}
	return d.nodes[identifier].Neighbors(), nil
}

// TraceNode returns a node's neighbors and all transitive neighbors using depth
// first search.
func (d *MapUpgradingDag) TraceNode(identifier string) (map[string]Node, error) {
	tree := map[string]Node{}
	if err := d.traceNode(identifier, tree); err != nil {
		return nil, err
	}
	return tree, nil
}

func (d *MapUpgradingDag) traceNode(identifier string, tree map[string]Node) error {
	if d.nodes[identifier] == nil {
		return errors.New("missing node in tree")
	}
	for _, n := range d.nodes[identifier].Neighbors() {
		// if we have already visited this neighbor, then we have already
		// visited its neighbors, so we can skip.
		if _, ok := tree[n.Identifier()]; ok {
			continue
		}
		tree[n.Identifier()] = n
		if err := d.traceNode(n.Identifier(), tree); err != nil {
			return err
		}
	}
	return nil
}

// GetNode returns a node in the dag.
func (d *MapUpgradingDag) GetNode(identifier string) (Node, error) {
	if _, ok := d.nodes[identifier]; !ok {
		return nil, errors.Errorf("node %s does not exist", identifier)
	}
	return d.nodes[identifier], nil
}

// AddEdges adds edges to the graph.
func (d *MapUpgradingDag) AddEdges(edges map[string][]Node) ([]Node, error) {
	var missing []Node
	for f, ne := range edges {
		for _, e := range ne {
			implied, err := d.AddEdge(f, e)
			if implied {
				missing = append(missing, e)
			}

			if err != nil {
				return nil, err
			}
		}
	}

	return missing, nil
}

// AddEdge adds an edge to the graph and returns if we need to check for updates.
func (d *MapUpgradingDag) AddEdge(from string, to Node) (bool, error) {
	if _, ok := d.nodes[from]; !ok {
		return false, errors.Errorf("node %s does not exist", to)
	}

	implied := false
	orgTo, ok := d.nodes[to.Identifier()]
	if !ok {
		implied = true
		if err := d.AddNode(to); err != nil {
			return implied, err
		}
	} else if !isValidConstraints(orgTo, to) { // check if upgrade is needed
		err := d.nodes[from].AddNeighbors(to)
		n := d.nodes[to.Identifier()]
		n.AddParentConstraints(to.GetParentConstraints())

		return true, err
	}

	err := d.nodes[from].AddNeighbors(to)
	d.nodes[to.Identifier()].AddParentConstraints(to.GetParentConstraints())

	return implied, err
}

// Sort performs topological sort on the graph.
func (d *MapUpgradingDag) Sort() ([]string, error) {
	visited := map[string]bool{}
	results := make([]string, len(d.nodes))
	for n, node := range d.nodes {
		if !visited[n] {
			stack := map[string]bool{}
			if err := d.visit(n, node.Neighbors(), stack, visited, results); err != nil {
				return nil, err
			}
		}
	}
	return results, nil
}

func (d *MapUpgradingDag) visit(name string, neighbors []Node, stack map[string]bool, visited map[string]bool, results []string) error {
	visited[name] = true
	stack[name] = true
	for _, n := range neighbors {
		if !visited[n.Identifier()] {
			if _, ok := d.nodes[n.Identifier()]; !ok {
				return errors.Errorf("node %q does not exist", n.Identifier())
			}
			if err := d.visit(n.Identifier(), d.nodes[n.Identifier()].Neighbors(), stack, visited, results); err != nil {
				return err
			}
		} else if stack[n.Identifier()] {
			return errors.Errorf("detected cycle on: %s", n.Identifier())
		}
	}
	for i, r := range results {
		if r == "" {
			results[i] = name
			break
		}
	}
	stack[name] = false
	return nil
}

func isValidConstraints(installed, wanted Node) bool {
	// NOTE(ezgidemirel): This condition also satisfies digests
	if installed.GetConstraints() == wanted.GetConstraints() {
		return true
	}

	c, err := semver.NewConstraint(wanted.GetConstraints())
	if err != nil {
		return false
	}

	v, err := semver.NewVersion(installed.GetConstraints())
	if err != nil {
		return false
	}

	if !c.Check(v) {
		return false
	}

	return true
}
