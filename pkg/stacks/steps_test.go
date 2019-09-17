/*
Copyright 2019 The Crossplane Authors.

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

package stacks

import (
	"testing"
)

func TestIconStep(t *testing.T) {
	sp := NewStackPackage("/")
	step := iconStep(sp)
	step("/icon.png", []byte("base64me"))

	if len(sp.Icons) != 1 {
		t.Errorf("iconStep(...); expected 1 icon")
	}

	if v, found := sp.Icons["/icon.png"]; !found {
		t.Errorf("iconStep(...); icon not found in StackPackage Icons")
	} else {
		if v.Base64IconData != "YmFzZTY0bWU=" {
			t.Errorf("iconStep(...); Base64IconData does not match")
		}

		if v.MediaType != "image/png" {
			t.Errorf("iconStep(...); MediaType does not match")
		}
	}

	if len(sp.Stack.Spec.AppMetadataSpec.Icons) == 0 {
		t.Errorf("iconStep(...); icon not found in Stack spec")
	} else {
		if sp.Stack.Spec.AppMetadataSpec.Icons[0].Base64IconData != "YmFzZTY0bWU=" {
			t.Errorf("iconStep(...); AppMetadataSpec Base64IconData does not match")
		}
		if sp.Stack.Spec.AppMetadataSpec.Icons[0].MediaType != "image/png" {
			t.Errorf("iconStep(...); AppMetadataSpec Base64IconData does not match")
		}
	}
}
