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

package dag

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

type simpleNode struct {
	identifier string
	neighbors  map[string]simpleNode
}

func (s *simpleNode) Identifier() string {
	return s.identifier
}

func (s *simpleNode) Neighbors() []Node {
	nodes := make([]Node, len(s.neighbors))
	i := 0
	for _, r := range s.neighbors {
		nodes[i] = &r
		i++
	}
	return nodes
}

func (s *simpleNode) AddNeighbors(nodes ...Node) error {
	for _, n := range nodes {
		sn, ok := n.(*simpleNode)
		if !ok {
			return errors.New("not a simple node")
		}
		s.neighbors[sn.Identifier()] = *sn
	}
	return nil
}

func toNodes(n []simpleNode) []Node {
	nodes := make([]Node, len(n))
	for i, r := range n {
		nodes[i] = &r
	}
	return nodes
}

var (
	_ DAG      = &MapDag{}
	_ NewDAGFn = NewMapDag
)

func sortedFnNop([]simpleNode, []string) error {
	return nil
}

func TestSort(t *testing.T) {
	one := "crossplane/one"
	two := "crossplane/two"
	three := "crossplane/three"
	four := "crossplane/four"
	five := "crossplane/five"
	six := "crossplane/six"
	seven := "crossplane/seven"
	eight := "crossplane/eight"
	nine := "crossplane/nine"
	dag := NewMapDag()
	type want struct {
		numImplied int
		numDeps    int
		sortedFn   func([]simpleNode, []string) error
		initErr    bool
		sortErr    bool
	}
	cases := map[string]struct {
		reason string
		nodes  []simpleNode
		want   want
	}{
		"Imply": {
			reason: "Missing nodes in a tree should be implied.",
			nodes: []simpleNode{
				{
					identifier: one,
					neighbors:  map[string]simpleNode{two: {identifier: two}, three: {identifier: three}},
				},
			},
			want: want{
				numImplied: 2,
				sortedFn:   sortedFnNop,
				numDeps:    2,
			},
		},
		"SimpleTree": {
			reason: "A dependency tree with one valid order should always be sorted the same.",
			nodes: []simpleNode{
				{
					identifier: one,
					neighbors:  map[string]simpleNode{two: {identifier: two}, three: {identifier: three}},
				},
				{
					identifier: two,
					neighbors:  map[string]simpleNode{three: {identifier: three}},
				},
				{
					identifier: three,
					neighbors:  map[string]simpleNode{four: {identifier: four}},
				},
				{
					identifier: four,
					neighbors:  map[string]simpleNode{five: {identifier: five}},
				},
				{
					identifier: five,
					neighbors:  map[string]simpleNode{six: {identifier: six}},
				},
				{
					identifier: six,
				},
			},
			want: want{
				sortedFn: func(nodes []simpleNode, sorted []string) error {
					for i, s := range sorted {
						if s != nodes[len(nodes)-i-1].Identifier() {
							errors.Errorf("Wrong sort: expected %s to be %s", s, nodes[len(nodes)-i-1].Identifier())
						}
					}
					return nil
				},
				numDeps: 5,
			},
		},
		"ComplexTree": {
			reason: "A dependency tree with multiple valid orders should always produce a valid order.",
			nodes: []simpleNode{
				{
					identifier: one,
					neighbors:  map[string]simpleNode{two: {identifier: two}, three: {identifier: three}},
				},
				{
					identifier: two,
					neighbors:  map[string]simpleNode{three: {identifier: three}, seven: {identifier: seven}, eight: {identifier: eight}, nine: {identifier: nine}},
				},
				{
					identifier: three,
					neighbors:  map[string]simpleNode{four: {identifier: four}},
				},
				{
					identifier: four,
					neighbors:  map[string]simpleNode{five: {identifier: five}},
				},
				{
					identifier: five,
					neighbors:  map[string]simpleNode{six: {identifier: six}},
				},
				{
					identifier: six,
					neighbors:  map[string]simpleNode{seven: {identifier: seven}, eight: {identifier: eight}, nine: {identifier: nine}},
				},
			},
			want: want{
				numImplied: 3,
				sortedFn: func(_ []simpleNode, sorted []string) error {
					indexMap := map[string]int{}
					for i, s := range sorted {
						indexMap[s] = i
					}
					if (indexMap[two] <= indexMap[four]) || (indexMap[four] <= indexMap[seven]) || (indexMap[four] <= indexMap[eight]) || (indexMap[four] <= indexMap[nine]) {
						return errors.Errorf("unexpected sorted order: %+v", sorted)
					}
					return nil
				},
				numDeps: 8,
			},
		},
		"Cycle": {
			reason: "A dependency tree with a cycle should return error when sorted.",
			nodes: []simpleNode{
				{
					identifier: one,
					neighbors:  map[string]simpleNode{two: {identifier: two}},
				},
				{
					identifier: two,
					neighbors:  map[string]simpleNode{one: {identifier: one}},
				},
			},
			want: want{
				sortErr: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			implied, err := dag.Init(toNodes(tc.nodes))
			if tc.want.initErr {
				if err == nil {
					t.Errorf("\n%s\nInit(...): expected error", tc.reason)
				}
				return
			}
			if diff := cmp.Diff(tc.want.numImplied, len(implied)); diff != "" {
				t.Errorf("\n%s\nimplied(...): -want, +got:\n%s", tc.reason, diff)
			}
			sorted, err := dag.Sort()
			if tc.want.sortErr {
				if err == nil {
					t.Errorf("\n%s\nSort(...): expected error", tc.reason)
				}
				return
			}
			if err := tc.want.sortedFn(tc.nodes, sorted); err != nil {
				t.Errorf("\n%s\nsorted(...): %s", tc.reason, err)
			}
			tree, _ := dag.TraceNode(tc.nodes[0].Identifier())
			if diff := cmp.Diff(tc.want.numDeps, len(tree)); diff != "" {
				t.Errorf("\n%s\\TraceNode(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDag(_ *testing.T) {
	d := NewMapDag()
	d.AddNode(&simpleNode{identifier: "hi"})
}
