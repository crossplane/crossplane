/*
Copyright 2021 The Crossplane Authors.

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

package composition

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha2"
)

func TestNewCompositionRevision(t *testing.T) {
	comp := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "coolcomp",
		},
	}

	compManifest, _ := json.Marshal(comp)

	var (
		rev  int64 = 1
		hash       = "1af1dfa857bf1d8814fe1af8983c18080019922e557f15a8a0d3db739d77aacb"
	)

	ctrl := true
	want := &v1alpha2.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", comp.GetName(), hash[0:7]),
			Labels: map[string]string{
				v1alpha2.LabelCompositionName: comp.GetName(),
				v1alpha2.LabelCompositionHash: hash[0:63],
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         v1.SchemeGroupVersion.String(),
				Kind:               v1.CompositionKind,
				Name:               comp.GetName(),
				Controller:         &ctrl,
				BlockOwnerDeletion: &ctrl,
			}},
		},
		// NOTE(negz): We're intentionally not testing most of the spec of this
		// type. We don't want to test generated code, and we've historically
		// demonstrated that it's tough to remember to update these conversion
		// tests when new fields are added to a type.
		Spec: v1alpha2.CompositionRevisionSpec{
			Revision: rev,
			Composition: extv1.JSON{
				Raw: compManifest,
			},
		},
	}

	got, _ := NewCompositionRevision(comp, rev, hash)
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("NewCompositionRevision(): -want, +got:\n%s", diff)
	}
}
