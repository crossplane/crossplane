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

package manager

import "strings"

func truncate(str string, num int) string {
	t := str
	if len(str) > num {
		t = str[0:num]
	}
	return t
}

// packNHash builds a maximum 63 character string made up of the name of a
// package and the hash of the its current revision.
func packNHash(provider, hash string) string {
	return strings.Join([]string{truncate(provider, 50), truncate(hash, 12)}, "-")
}

// imageToPod converts a valid image name to valid pod name.
func imageToPod(img string) string {
	return strings.NewReplacer("/", "-", ":", "-").Replace(img)
}
