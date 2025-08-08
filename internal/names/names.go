/*
Copyright 2025 The Crossplane Authors.

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

// Inspired by https://github.com/knative/pkg/blob/ee1db869c7ef25eb4ac5c9ba0ab73fdc3f1b9dfa/kmeta/names.go

package names

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	// maxKubernetesNameLength is the maximum length allowed for Kubernetes resource names.
	maxKubernetesNameLength = 63
	// hashLength is the length of the hash suffix we append to child names.
	hashLength = 12
)

// ChildName generates a name for a child resource based on the parent name, parent UID,
// and child name. The result follows the format: <parentname>-<hash> where the hash
// is derived from the parent UID and child name to ensure uniqueness.
// If the parent name doesn't end with a hyphen, one is added before the hash.
// If the full name would exceed Kubernetes' 63-character limit, the parent name
// is truncated to fit.
func ChildName(parentName, parentUID, childName string) string {
	// Create hash of parent UID + child name
	h := sha256.Sum256([]byte(parentUID + childName))
	suffix := hex.EncodeToString(h[:])[:hashLength]

	// Ensure parent ends with exactly one hyphen
	if parentName != "" && !strings.HasSuffix(parentName, "-") {
		parentName += "-"
	}

	// If parent + suffix fits, use it
	fullName := parentName + suffix
	if len(fullName) <= maxKubernetesNameLength {
		return fullName
	}

	// Otherwise truncate parent to fit (ensuring we leave room for the hyphen if needed)
	maxParentLen := maxKubernetesNameLength - hashLength
	if maxParentLen > 0 && !strings.HasSuffix(parentName[:maxParentLen], "-") {
		// Need room for a hyphen
		maxParentLen--
		truncatedParent := parentName[:maxParentLen] + "-"
		return truncatedParent + suffix
	}

	truncatedParent := parentName[:maxParentLen]
	return truncatedParent + suffix
}
