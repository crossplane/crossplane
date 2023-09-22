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

package features

import (
	"github.com/alecthomas/kong"
)

// maturityTag is the struct field tag used to specify maturity of a command.
const maturityTag = "maturity"

// Maturity is the maturity of a feature.
type Maturity string

// Currently supported maturity levels.
const (
	Alpha  Maturity = "alpha"
	Stable Maturity = "stable"
)

// HideMaturity hides commands that are not at the specified level of maturity.
func HideMaturity(p *kong.Path, maturity Maturity) error {
	nodes := p.Node().Children // copy to avoid possibility of reslicing
	nodes = append(nodes, p.Node())
	for _, c := range nodes {
		mt := Maturity(c.Tag.Get(maturityTag))
		if mt == "" {
			mt = Stable
		}
		if mt != maturity {
			c.Hidden = true
		}
	}
	return nil
}

// GetMaturity gets the maturity of the node.
func GetMaturity(n *kong.Node) Maturity {
	if m := Maturity(n.Tag.Get(maturityTag)); m != "" {
		return m
	}
	return Stable
}
