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

package dag

import (
	"errors"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

type SimpleFuzzNode struct {
	IdentifierString string
	NeighborsField   map[string]SimpleFuzzNode
}

func toNodesFuzz(n []SimpleFuzzNode) []Node {
	nodes := make([]Node, len(n))
	for i, r := range n {
		nodes[i] = &r
	}
	return nodes
}

func (s *SimpleFuzzNode) AddNeighbors(nodes ...Node) error {
	for _, n := range nodes {
		sn, ok := n.(*SimpleFuzzNode)
		if !ok {
			return errors.New("not a simple node")
		}
		if s.NeighborsField == nil {
			s.NeighborsField = make(map[string]SimpleFuzzNode)
		}
		s.NeighborsField[sn.Identifier()] = *sn
	}
	return nil
}

func (s *SimpleFuzzNode) Identifier() string {
	return s.IdentifierString
}

func (s *SimpleFuzzNode) Neighbors() []Node {
	nodes := make([]Node, len(s.NeighborsField))
	i := 0
	for _, r := range s.NeighborsField {
		nodes[i] = &r
		i++
	}
	return nodes
}

func FuzzDag(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte) {
		c := fuzz.NewConsumer(data)
		nodes := make([]SimpleFuzzNode, 0)
		err := c.CreateSlice(&nodes)
		if err != nil {
			return
		}
		for _, n := range nodes {
			if n.NeighborsField == nil {
				n.NeighborsField = make(map[string]SimpleFuzzNode)
			}
		}
		d := NewMapDag()

		_, _ = d.Init(toNodesFuzz(nodes))
		identifier, err := c.GetString()
		if err != nil {
			return
		}
		d.Sort()
		_, _ = d.TraceNode(identifier)
		d.Sort()
		from, err := c.GetString()
		if err != nil {
			return
		}
		fuzzNode := &SimpleFuzzNode{}
		c.GenerateStruct(fuzzNode)
		_, _ = d.AddEdge(from, fuzzNode)
		d.Sort()
		d.NodeNeighbors(identifier)
	})
}
