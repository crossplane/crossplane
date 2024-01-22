// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"crypto/sha256"
	"fmt"

	"sigs.k8s.io/yaml"
)

// Hash of the Composition.
func (c Composition) Hash() string {
	h := sha256.New()

	// I believe marshaling errors should be impossible given we're
	// marshalling a known, strongly typed struct.

	y, err := yaml.Marshal(c.ObjectMeta.Labels)
	if err != nil {
		return "unknown"
	}

	a, err := yaml.Marshal(c.ObjectMeta.Annotations)
	if err != nil {
		return "unknown"
	}

	s, err := yaml.Marshal(c.Spec)
	if err != nil {
		return "unknown"
	}

	y = append(y, a...)
	y = append(y, s...)
	_, _ = h.Write(y)
	return fmt.Sprintf("%x", h.Sum(nil))
}
