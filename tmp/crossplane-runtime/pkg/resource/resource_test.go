/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    htcp://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

const (
	namespace = "coolns"
	name      = "cool"
	uid       = types.UID("definitely-a-uuid")
	testSteps = 3
)

var (
	MockOwnerGVK = schema.GroupVersionKind{
		Group:   "cool",
		Version: "large",
		Kind:    "MockOwner",
	}

	testBackoff = wait.Backoff{}
	errTest     = errors.New("test-error")
)

func TestLocalConnectionSecretFor(t *testing.T) {
	secretName := "coolsecret"

	type args struct {
		o    LocalConnectionSecretOwner
		kind schema.GroupVersionKind
	}

	controller := true

	cases := map[string]struct {
		args args
		want *corev1.Secret
	}{
		"Success": {
			args: args{
				o: &fake.MockLocalConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
						UID:       uid,
					},
					Ref: &xpv1.LocalSecretReference{Name: secretName},
				},
				kind: MockOwnerGVK,
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      secretName,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         MockOwnerGVK.GroupVersion().String(),
						Kind:               MockOwnerGVK.Kind,
						Name:               name,
						UID:                uid,
						Controller:         &controller,
						BlockOwnerDeletion: &controller,
					}},
				},
				Type: SecretTypeConnection,
				Data: map[string][]byte{},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := LocalConnectionSecretFor(tc.args.o, tc.args.kind)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("LocalConnectionSecretFor(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnectionSecretFor(t *testing.T) {
	secretName := "coolsecret"

	type args struct {
		o    ConnectionSecretOwner
		kind schema.GroupVersionKind
	}

	controller := true

	cases := map[string]struct {
		args args
		want *corev1.Secret
	}{
		"Success": {
			args: args{
				o: &fake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
						UID:       uid,
					},
					WriterTo: &xpv1.SecretReference{Namespace: namespace, Name: secretName},
				},
				kind: MockOwnerGVK,
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      secretName,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         MockOwnerGVK.GroupVersion().String(),
						Kind:               MockOwnerGVK.Kind,
						Name:               name,
						UID:                uid,
						Controller:         &controller,
						BlockOwnerDeletion: &controller,
					}},
				},
				Type: SecretTypeConnection,
				Data: map[string][]byte{},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ConnectionSecretFor(tc.args.o, tc.args.kind)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ConnectionSecretFor(): -want, +got:\n%s", diff)
			}
		})
	}
}

type MockTyper struct {
	GVKs        []schema.GroupVersionKind
	Unversioned bool
	Error       error
}

func (t MockTyper) ObjectKinds(_ runtime.Object) ([]schema.GroupVersionKind, bool, error) {
	return t.GVKs, t.Unversioned, t.Error
}

func (t MockTyper) Recognizes(_ schema.GroupVersionKind) bool { return true }

func TestGetKind(t *testing.T) {
	type args struct {
		obj runtime.Object
		ot  runtime.ObjectTyper
	}

	type want struct {
		kind schema.GroupVersionKind
		err  error
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		args args
		want want
	}{
		"KindFound": {
			args: args{
				ot: MockTyper{GVKs: []schema.GroupVersionKind{fake.GVK(&fake.Managed{})}},
			},
			want: want{
				kind: fake.GVK(&fake.Managed{}),
			},
		},
		"KindError": {
			args: args{
				ot: MockTyper{Error: errBoom},
			},
			want: want{
				err: errors.Wrap(errBoom, "cannot get kind of supplied object"),
			},
		},
		"KindIsUnversioned": {
			args: args{
				ot: MockTyper{Unversioned: true},
			},
			want: want{
				err: errors.New("supplied object is unversioned"),
			},
		},
		"NotEnoughKinds": {
			args: args{
				ot: MockTyper{},
			},
			want: want{
				err: errors.New("supplied object does not have exactly one kind"),
			},
		},
		"TooManyKinds": {
			args: args{
				ot: MockTyper{GVKs: []schema.GroupVersionKind{
					fake.GVK(&fake.Object{}),
					fake.GVK(&fake.Managed{}),
				}},
			},
			want: want{
				err: errors.New("supplied object does not have exactly one kind"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := GetKind(tc.args.obj, tc.args.ot)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("GetKind(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.kind, got); diff != "" {
				t.Errorf("GetKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestMustCreateObject(t *testing.T) {
	type args struct {
		kind schema.GroupVersionKind
		oc   runtime.ObjectCreater
	}

	cases := map[string]struct {
		args args
		want runtime.Object
	}{
		"KindRegistered": {
			args: args{
				kind: fake.GVK(&fake.Managed{}),
				oc:   fake.SchemeWith(&fake.Managed{}),
			},
			want: &fake.Managed{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := MustCreateObject(tc.args.kind, tc.args.oc)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MustCreateObject(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIgnore(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		is  ErrorIs
		err error
	}

	cases := map[string]struct {
		args args
		want error
	}{
		"IgnoreError": {
			args: args{
				is:  func(_ error) bool { return true },
				err: errBoom,
			},
			want: nil,
		},
		"PropagateError": {
			args: args{
				is:  func(_ error) bool { return false },
				err: errBoom,
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Ignore(tc.args.is, tc.args.err)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Ignore(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestIgnoreAny(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		is  []ErrorIs
		err error
	}

	cases := map[string]struct {
		args args
		want error
	}{
		"IgnoreError": {
			args: args{
				is:  []ErrorIs{func(_ error) bool { return true }},
				err: errBoom,
			},
			want: nil,
		},
		"IgnoreErrorArr": {
			args: args{
				is: []ErrorIs{
					func(_ error) bool { return true },
					func(_ error) bool { return false },
				},
				err: errBoom,
			},
			want: nil,
		},
		"PropagateError": {
			args: args{
				is:  []ErrorIs{func(_ error) bool { return false }},
				err: errBoom,
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IgnoreAny(tc.args.err, tc.args.is...)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Ignore(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestIsConditionTrue(t *testing.T) {
	cases := map[string]struct {
		c    xpv1.Condition
		want bool
	}{
		"IsTrue": {
			c:    xpv1.Condition{Status: corev1.ConditionTrue},
			want: true,
		},
		"IsFalse": {
			c:    xpv1.Condition{Status: corev1.ConditionFalse},
			want: false,
		},
		"IsUnknown": {
			c:    xpv1.Condition{Status: corev1.ConditionUnknown},
			want: false,
		},
		"IsUnset": {
			c:    xpv1.Condition{},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsConditionTrue(tc.c)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsConditionTrue(...): -want, +got:\n%s", diff)
			}
		})
	}
}

type object struct {
	runtime.Object
	metav1.ObjectMeta
}

func (o *object) DeepCopyObject() runtime.Object {
	return &object{ObjectMeta: *o.DeepCopy()}
}

func TestIsNotControllable(t *testing.T) {
	cases := map[string]struct {
		reason string
		err    error
		want   bool
	}{
		"NilError": {
			reason: "A nil error does not indicate something is not controllable.",
			err:    nil,
			want:   false,
		},
		"UnknownError": {
			reason: "An that doesn't have a 'NotControllable() bool' method does not indicate something is not controllable.",
			err:    errors.New("boom"),
			want:   false,
		},
		"NotControllableError": {
			reason: "An that has a 'NotControllable() bool' method indicates something is not controllable.",
			err:    notControllableError{errors.New("boom")},
			want:   true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsNotControllable(tc.err)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nIsNotControllable(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestMustBeControllableBy(t *testing.T) {
	uid := types.UID("very-unique-string")
	controller := true

	type args struct {
		ctx     context.Context
		current runtime.Object
		desired runtime.Object
	}

	cases := map[string]struct {
		reason string
		u      types.UID
		args   args
		want   error
	}{
		"Adoptable": {
			reason: "A current object with no controller reference may be adopted and controlled",
			u:      uid,
			args: args{
				current: &object{},
			},
		},
		"ControlledBySuppliedUID": {
			reason: "A current object that is already controlled by the supplied UID is controllable",
			u:      uid,
			args: args{
				current: &object{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					UID:        uid,
					Controller: &controller,
				}}}},
			},
		},
		"ControlledBySomeoneElse": {
			reason: "A current object that is already controlled by a different UID is not controllable",
			u:      uid,
			args: args{
				current: &object{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					UID:        types.UID("some-other-uid"),
					Controller: &controller,
				}}}},
			},
			want: notControllableError{errors.Errorf("existing object is not controlled by UID %q", uid)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ao := MustBeControllableBy(tc.u)

			err := ao(tc.args.ctx, tc.args.current, tc.args.desired)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMustBeControllableBy(...)(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestConnectionSecretMustBeControllableBy(t *testing.T) {
	uid := types.UID("very-unique-string")
	controller := true

	type args struct {
		ctx     context.Context
		current runtime.Object
		desired runtime.Object
	}

	cases := map[string]struct {
		reason string
		u      types.UID
		args   args
		want   error
	}{
		"Adoptable": {
			reason: "A Secret of SecretTypeConnection with no controller reference may be adopted and controlled",
			u:      uid,
			args: args{
				current: &corev1.Secret{Type: SecretTypeConnection},
			},
		},
		"ControlledBySuppliedUID": {
			reason: "A Secret of any type that is already controlled by the supplied UID is controllable",
			u:      uid,
			args: args{
				current: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
						UID:        uid,
						Controller: &controller,
					}}},
					Type: corev1.SecretTypeOpaque,
				},
			},
		},
		"ControlledBySomeoneElse": {
			reason: "A Secret of any type that is already controlled by the another UID is not controllable",
			u:      uid,
			args: args{
				current: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
						UID:        types.UID("some-other-uid"),
						Controller: &controller,
					}}},
					Type: SecretTypeConnection,
				},
			},
			want: notControllableError{errors.Errorf("existing secret is not controlled by UID %q", uid)},
		},
		"UncontrolledOpaqueSecret": {
			reason: "A Secret of corev1.SecretTypeOpqaue with no controller is not controllable",
			u:      uid,
			args: args{
				current: &corev1.Secret{Type: corev1.SecretTypeOpaque},
			},
			want: notControllableError{errors.Errorf("refusing to modify uncontrolled secret of type %q", corev1.SecretTypeOpaque)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ao := ConnectionSecretMustBeControllableBy(tc.u)

			err := ao(tc.args.ctx, tc.args.current, tc.args.desired)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConnectionSecretMustBeControllableBy(...)(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestAllowUpdateIf(t *testing.T) {
	type args struct {
		ctx     context.Context
		current runtime.Object
		desired runtime.Object
	}

	cases := map[string]struct {
		reason string
		fn     func(current, desired runtime.Object) bool
		args   args
		want   error
	}{
		"Allowed": {
			reason: "No error should be returned when the supplied function returns true",
			fn:     func(_, _ runtime.Object) bool { return true },
			args: args{
				current: &object{},
			},
		},
		"NotAllowed": {
			reason: "An error that satisfies IsNotAllowed should be returned when the supplied function returns false",
			fn:     func(_, _ runtime.Object) bool { return false },
			args: args{
				current: &object{},
			},
			want: notAllowedError{errors.New("update not allowed")},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ao := AllowUpdateIf(tc.fn)

			err := ao(tc.args.ctx, tc.args.current, tc.args.desired)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAllowUpdateIf(...)(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestGetExternalTags(t *testing.T) {
	provName := "prov"
	cases := map[string]struct {
		o    Managed
		want map[string]string
	}{
		"SuccessfulWithTypedProviderConfig": {
			o: &fake.ModernManaged{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				TypedProviderConfigReferencer: fake.TypedProviderConfigReferencer{
					Ref: &xpv1.ProviderConfigReference{Name: provName, Kind: "ProviderConfig"},
				},
			},
			want: map[string]string{
				ExternalResourceTagKeyKind:               strings.ToLower((&fake.Managed{}).GetObjectKind().GroupVersionKind().GroupKind().String()),
				ExternalResourceTagKeyName:               name,
				ExternalResourceTagKeyProvider:           provName,
				ExternalResourceTagKeyProviderConfigKind: "ProviderConfig",
			},
		},
		"SuccessfulWithNamespacedObject": {
			o: &fake.ModernManaged{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				TypedProviderConfigReferencer: fake.TypedProviderConfigReferencer{
					Ref: &xpv1.ProviderConfigReference{Name: provName, Kind: "ProviderConfig"},
				},
			},
			want: map[string]string{
				ExternalResourceTagKeyKind:               strings.ToLower((&fake.Managed{}).GetObjectKind().GroupVersionKind().GroupKind().String()),
				ExternalResourceTagKeyName:               name,
				ExternalResourceTagKeyNamespace:          namespace,
				ExternalResourceTagKeyProvider:           provName,
				ExternalResourceTagKeyProviderConfigKind: "ProviderConfig",
			},
		},
		"SuccessfulWithLegacyProviderConfig": {
			o: &fake.LegacyManaged{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				LegacyProviderConfigReferencer: fake.LegacyProviderConfigReferencer{Ref: &xpv1.Reference{Name: provName}},
			},
			want: map[string]string{
				ExternalResourceTagKeyKind:     strings.ToLower((&fake.LegacyManaged{}).GetObjectKind().GroupVersionKind().GroupKind().String()),
				ExternalResourceTagKeyName:     name,
				ExternalResourceTagKeyProvider: provName,
			},
		},
		"NotLegacyOrModernManaged": {
			o: &fake.Managed{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			},
			want: map[string]string{
				ExternalResourceTagKeyKind: strings.ToLower((&fake.Managed{}).GetObjectKind().GroupVersionKind().GroupKind().String()),
				ExternalResourceTagKeyName: name,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetExternalTags(tc.o)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("GetExternalTags(...): -want, +got:\n%s", diff)
			}
		})
	}
}

// single test case => not using tables.
func Test_notControllableError_NotControllable(t *testing.T) {
	err := notControllableError{
		errors.New("test-error"),
	}
	if !err.NotControllable() {
		t.Errorf("NotControllable(): false")
	}
}

// single test case => not using tables.
func Test_notAllowedError_NotAllowed(t *testing.T) {
	err := notAllowedError{
		errors.New("test-error"),
	}
	if !err.NotAllowed() {
		t.Errorf("NotAllowed(): false")
	}
}

func TestIsAPIErrorWrapped(t *testing.T) {
	testCases := map[string]struct {
		err  error
		want bool
	}{
		"NoError": {
			want: false,
		},
		"NotAPIError": {
			err:  errors.New("test-error"),
			want: false,
		},
		"APIError": {
			err:  kerrors.NewNotFound(schema.GroupResource{}, "test-resource"),
			want: true,
		},
		"WrappedAPIError": {
			err: errors.Wrap(
				kerrors.NewNotFound(schema.GroupResource{}, "test-resource"), "test-wrapper"),
			want: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := IsAPIErrorWrapped(tc.err); got != tc.want {
				t.Errorf("IsAPIErrorWrapped() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewApplicatorWithRetry(t *testing.T) {
	type args struct {
		applicator  Applicator
		shouldRetry shouldRetryFunc
		backoff     *wait.Backoff
	}

	testCases := map[string]struct {
		args args
		want Applicator
	}{
		"DefaultBackoff": {
			args: args{},
			want: &ApplicatorWithRetry{
				backoff: retry.DefaultRetry,
			},
		},
		"CustomBackoff": {
			args: args{
				backoff: &testBackoff,
			},
			want: &ApplicatorWithRetry{
				backoff: testBackoff,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want,
				NewApplicatorWithRetry(tc.args.applicator, tc.args.shouldRetry, tc.args.backoff),
				cmp.AllowUnexported(ApplicatorWithRetry{})); diff != "" {
				t.Errorf("NewApplicatorWithRetry(...): -want, +got:\n%s", diff)
			}
		})
	}
}

type mockApplicator struct {
	returnError bool
	count       uint
}

func (m *mockApplicator) Apply(_ context.Context, _ client.Object, _ ...ApplyOption) error {
	m.count++

	if m.returnError {
		return errTest
	}

	return nil
}

func TestApplicatorWithRetry_Apply(t *testing.T) {
	type fields struct {
		applicator  Applicator
		shouldRetry shouldRetryFunc
		backoff     wait.Backoff
	}

	type args struct {
		ctx  context.Context
		c    client.Object
		opts []ApplyOption
	}

	testCases := map[string]struct {
		fields    fields
		args      args
		wantErr   error
		wantCount uint
	}{
		"NoRetry": {
			fields: fields{
				applicator: &mockApplicator{returnError: true},
				shouldRetry: func(_ error) bool {
					return false
				},
				backoff: wait.Backoff{Steps: testSteps},
			},
			args:      args{},
			wantErr:   errTest,
			wantCount: 1,
		},
		"ShouldRetry": {
			fields: fields{
				applicator: &mockApplicator{returnError: true},
				shouldRetry: func(_ error) bool {
					return true
				},
				backoff: wait.Backoff{Steps: testSteps},
			},
			args:      args{},
			wantErr:   errTest,
			wantCount: testSteps,
		},
		"NoError": {
			fields: fields{
				applicator: &mockApplicator{},
				shouldRetry: func(_ error) bool {
					return true
				},
				backoff: wait.Backoff{Steps: testSteps},
			},
			args:      args{},
			wantErr:   nil,
			wantCount: 1,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			awr := &ApplicatorWithRetry{
				Applicator:  tc.fields.applicator,
				shouldRetry: tc.fields.shouldRetry,
				backoff:     tc.fields.backoff,
			}

			if diff := cmp.Diff(tc.wantErr, awr.Apply(tc.args.ctx, tc.args.c, tc.args.opts...), test.EquateErrors()); diff != "" {
				t.Fatalf("ApplicatorWithRetry.Apply(...): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(awr.Applicator.(*mockApplicator).count, tc.wantCount); diff != "" {
				t.Errorf("Retry count mismatch: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type args struct {
		fn      func(current, desired runtime.Object)
		current runtime.Object
		desired runtime.Object
	}

	tests := map[string]struct {
		args args
		want runtime.Object
	}{
		"Update": {
			args: args{
				fn: func(current, desired runtime.Object) {
					c, d := current.(*corev1.Secret), desired.(*corev1.Secret)

					c.StringData = d.StringData
				},
				current: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "current",
					},
				},
				desired: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "desired",
					},
					StringData: map[string]string{
						"key": "value",
					},
				},
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "current",
				},
				StringData: map[string]string{
					"key": "value",
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if err := UpdateFn(tt.args.fn)(nil, tt.args.current, tt.args.desired); err != nil {
				t.Fatalf("ApplyOption() = %v, want %v", err, nil)
			}

			if diff := cmp.Diff(tt.want, tt.args.current); diff != "" {
				t.Errorf("UpdateFn updated object mismatch: -want, +got: %s", diff)
			}
		})
	}
}

func TestFirstNAndSomeMore(t *testing.T) {
	type args struct {
		n     int
		names []string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{args: args{n: 3, names: []string{"a", "b", "c", "d", "e"}}, want: "a, b, c, and 2 more"},
		{args: args{n: 3, names: []string{"a", "b", "c"}}, want: "a, b, and c"},
		{args: args{n: 3, names: []string{"a", "b"}}, want: "a, b"},
		{args: args{n: 3, names: []string{"a"}}, want: "a"},
		{args: args{n: 3, names: []string{}}, want: ""},
		{args: args{n: 3, names: []string{"a", "c", "e", "b", "d"}}, want: "a, c, e, and 2 more"},
		{args: args{n: 3, names: []string{"a", "b", "b", "b", "d"}}, want: "a, b, b, and 2 more"}, //nolint:dupword // Intentional.
		{args: args{n: 2, names: []string{"a", "c", "e", "b", "d"}}, want: "a, c, and 3 more"},
		{args: args{n: 0, names: []string{"a", "c", "e", "b", "d"}}, want: "5"},
		{args: args{n: -7, names: []string{"a", "c", "e", "b", "d"}}, want: "5"},
		{args: args{n: 1, names: []string{"a", "c", "e", "b", "d"}}, want: "a, and 4 more"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FirstNAndSomeMore(tt.args.n, tt.args.names); got != tt.want {
				t.Errorf("FirstNAndSomeMore() = %v, want %v", got, tt.want)
			}
		})
	}
}
