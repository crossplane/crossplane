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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestNewCompositionRevision(t *testing.T) {
	comp := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "coolcomp",
		},
	}

	var (
		rev  int64 = 1
		hash       = "775265aaf20123c08e3eee3fc546e69c8f0bbff595a6030600dcf90fe9dd9ef"
	)

	ctrl := true
	want := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", comp.GetName(), hash[0:7]),
			Labels: map[string]string{
				v1.LabelCompositionName: comp.GetName(),
				v1.LabelCompositionHash: hash[0:63],
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
		Spec: v1.CompositionRevisionSpec{
			Revision: rev,
		},
	}

	got := NewCompositionRevision(comp, rev)
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("NewCompositionRevision(): -want, +got:\n%s", diff)
	}
}
