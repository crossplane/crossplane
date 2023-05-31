/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package composite

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestRender(t *testing.T) {
	ctrl := true
	tmpl, _ := json.Marshal(&fake.Managed{})
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		cp  resource.Composite
		cd  resource.Composed
		t   v1.ComposedTemplate
	}
	type want struct {
		cd  resource.Composed
		err error
	}
	cases := map[string]struct {
		reason string
		client client.Client
		args
		want
	}{
		"InvalidTemplate": {
			reason: "Invalid template should not be accepted",
			args: args{
				cd: &fake.Composed{},
				t:  v1.ComposedTemplate{Base: runtime.RawExtension{Raw: []byte("olala")}},
			},
			want: want{
				cd:  &fake.Composed{},
				err: errors.Wrap(errors.New("invalid character 'o' looking for beginning of value"), errUnmarshal),
			},
		},
		"NoLabel": {
			reason: "The name prefix label has to be set",
			args: args{
				cp: &fake.Composite{},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
				t:  v1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd:  &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
				err: errors.New(errNamePrefix),
			},
		},
		"DryRunError": {
			reason: "Errors dry-run creating the rendered resource to name it should be returned",
			client: &test.MockClient{MockCreate: test.NewMockCreateFn(errBoom)},
			args: args{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					xcrd.LabelKeyNamePrefixForComposed: "ola",
					xcrd.LabelKeyClaimName:             "rola",
					xcrd.LabelKeyClaimNamespace:        "rolans",
				}}},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{}},
				t:  v1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ola-",
					Labels: map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "ola",
						xcrd.LabelKeyClaimName:             "rola",
						xcrd.LabelKeyClaimNamespace:        "rolans",
					},
					OwnerReferences: []metav1.OwnerReference{{Controller: &ctrl, BlockOwnerDeletion: &ctrl}},
				}},
				err: errors.Wrap(errBoom, errName),
			},
		},
		"ControllerError": {
			reason: "External controller owner references should cause an exception",
			client: &test.MockClient{MockCreate: test.NewMockCreateFn(nil)},
			args: args{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					xcrd.LabelKeyNamePrefixForComposed: "ola",
					xcrd.LabelKeyClaimName:             "rola",
					xcrd.LabelKeyClaimNamespace:        "rolans",
				}}},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd",
					OwnerReferences: []metav1.OwnerReference{{Controller: &ctrl, BlockOwnerDeletion: &ctrl,
						UID: "random_uid"}}}},
				t: v1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					Name:         "cd",
					GenerateName: "ola-",
					Labels: map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "ola",
						xcrd.LabelKeyClaimName:             "rola",
						xcrd.LabelKeyClaimNamespace:        "rolans",
					},
					OwnerReferences: []metav1.OwnerReference{{Controller: &ctrl, BlockOwnerDeletion: &ctrl,
						UID: "random_uid"}},
				}},
				err: errors.Wrap(errors.Errorf("cd is already controlled by   (UID random_uid)"), errSetControllerRef),
			},
		},
		"Success": {
			reason: "Configuration should result in the right object with correct generateName",
			client: &test.MockClient{MockCreate: test.NewMockCreateFn(nil)},
			args: args{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					xcrd.LabelKeyNamePrefixForComposed: "ola",
					xcrd.LabelKeyClaimName:             "rola",
					xcrd.LabelKeyClaimNamespace:        "rolans",
				}}},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
				t:  v1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					Name:         "cd",
					GenerateName: "ola-",
					Labels: map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "ola",
						xcrd.LabelKeyClaimName:             "rola",
						xcrd.LabelKeyClaimNamespace:        "rolans",
					},
					OwnerReferences: []metav1.OwnerReference{{Controller: &ctrl, BlockOwnerDeletion: &ctrl}},
				}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPIDryRunRenderer(tc.client)
			err := r.Render(tc.args.ctx, tc.args.cp, tc.args.cd, tc.args.t, nil)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cd, tc.args.cd); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
