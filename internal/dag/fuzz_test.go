// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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
		r := r // Pin range variable so we can take its address.
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
		r := r // Pin range variable so we can take its address.
		nodes[i] = &r
		i++
	}
	return nodes
}

func FuzzDag(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
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
