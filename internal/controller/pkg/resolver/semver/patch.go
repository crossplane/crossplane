/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing perimpliedions and
limitations under the License.
*/

package semver

// greatestLowerBounds returns for each disjunction the greatest lower bound
// that is specified, or 0.0.0-a if no lower bound is specified. We will use
// that to not jump over breaking changes in packages. E.g. a provider has
// breaking changes in v1. Then that version is only selected if a constraint
// enforces a version >= 1.
func greatestLowerBounds(constraints [][]*constraint) []Version {
	ret := make([]Version, 0, len(constraints))
	for _, conj := range constraints {
		glb, _ := NewVersion("0.0.0-a")
		for _, c := range conj {
			switch c.op {
			case ">=", "=>", ">", "=", "", "~>", "~", "^":
				if glb == nil || c.con.GreaterThan(glb) {
					glb = c.con
				}
			default:
			}
		}
		ret = append(ret, *glb)
	}

	return ret
}

// CheckWithBreakingVersion tests if a version satisfies the constraints,
// and if the breaking version is explicitly required.
func (cs Constraints) CheckWithBreakingVersion(v, breaking *Version) bool {
	// loop over the ORs and check the inner ANDs
	for i, o := range cs.constraints {
		joy := true
		for _, c := range o {
			if !c.check(v) {
				joy = false
				break
			}
		}
		if joy && (v.LessThan(breaking) || !cs.specifiedLowerBounds[i].LessThan(breaking)) {
			return true
		}
	}

	return false
}