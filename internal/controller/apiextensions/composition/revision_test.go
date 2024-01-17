// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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
