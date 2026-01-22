package reference

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func prepareTestExamplesNamespaced(numExamples int) ([]string, []xpv1.NamespacedReference, []*fake.Managed) {
	values := make([]string, numExamples)
	refs := make([]xpv1.NamespacedReference, numExamples)
	controlledObj := make([]*fake.Managed, numExamples)
	for i := range numExamples {
		values[i] = fmt.Sprintf("%s%d", testValuePrefix, i)
		refs[i] = xpv1.NamespacedReference{
			Name: fmt.Sprintf("%s%d", testResourceNamePrefix, i),
		}
		controlled := &fake.Managed{}
		controlled.SetName(refs[i].Name)
		meta.SetExternalName(controlled, values[i])
		_ = meta.AddControllerReference(controlled, meta.AsController(&xpv1.TypedReference{UID: testControllerUID}))
		controlledObj[i] = controlled
	}
	return values, refs, controlledObj
}

var nsTestValues, nsTestRefs, nsTestControlled = prepareTestExamplesNamespaced(10)

func TestNamespacedResolve(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()
	value := "coolv"
	ref := &xpv1.NamespacedReference{Name: "cool", Namespace: "cool-ns"}
	nsOmittedRef := &xpv1.NamespacedReference{Name: "cool"}
	optionalPolicy := xpv1.ResolutionPolicyOptional
	alwaysPolicy := xpv1.ResolvePolicyAlways
	optionalRef := &xpv1.NamespacedReference{Name: "cool", Namespace: "cool-ns", Policy: &xpv1.Policy{Resolution: &optionalPolicy}}
	alwaysRef := &xpv1.NamespacedReference{Name: "cool", Namespace: "cool-ns", Policy: &xpv1.Policy{Resolve: &alwaysPolicy}}

	controlled := &fake.Managed{}
	controlled.SetName(value)
	meta.SetExternalName(controlled, value)
	meta.AddControllerReference(controlled, meta.AsController(&xpv1.TypedReference{UID: types.UID("very-unique")}))

	type args struct {
		ctx context.Context
		req NamespacedResolutionRequest
	}

	type want struct {
		rsp NamespacedResolutionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Reader
		from   resource.Managed
		args   args
		want   want
	}{
		"FromDeleted": {
			reason: "Should return early if the referencing managed resource was deleted",
			from:   &fake.Managed{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}},
			args: args{
				req: NamespacedResolutionRequest{},
			},
			want: want{
				rsp: NamespacedResolutionResponse{},
				err: nil,
			},
		},
		"AlreadyResolved": {
			reason: "Should return early if the current value is non-zero",
			from:   &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{CurrentValue: value},
			},
			want: want{
				rsp: NamespacedResolutionResponse{ResolvedValue: value},
				err: nil,
			},
		},
		"AlwaysResolveReference": {
			reason: "Should not return early if the current value is non-zero, when the resolve policy is set to" +
				"Always",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					meta.SetExternalName(obj.(metav1.Object), value)
					return nil
				}),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Reference:    alwaysRef,
					To:           To{Managed: &fake.Managed{}},
					Extract:      ExternalName(),
					CurrentValue: "oldValue",
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: alwaysRef,
				},
				err: nil,
			},
		},
		"Unresolvable": {
			reason: "Should return early if neither a reference or selector were provided",
			from:   &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{},
			},
			want: want{
				err: nil,
			},
		},
		"GetError": {
			reason: "Should return errors encountered while getting the referenced resource",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Reference: ref,
					To:        To{Managed: &fake.Managed{}},
					Extract:   ExternalName(),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetManaged),
			},
		},
		"ResolvedNoValue": {
			reason: "Should return an error if the extract function returns the empty string",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Reference: ref,
					To:        To{Managed: &fake.Managed{}},
					Extract:   func(resource.Managed) string { return "" },
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedReference: ref,
				},
				err: errors.New(errNoValue),
			},
		},
		"SuccessfulResolve": {
			reason: "No error should be returned when the value is successfully extracted",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					meta.SetExternalName(obj.(metav1.Object), value)
					return nil
				}),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Reference: ref,
					To:        To{Managed: &fake.Managed{}},
					Extract:   ExternalName(),
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: ref,
				},
			},
		},
		"SuccessfulResolveNamespaced": {
			reason: "Resolve should be successful when a namespace is given",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					meta.SetExternalName(obj.(metav1.Object), value)
					return nil
				}),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Reference: ref,
					To:        To{Managed: &fake.Managed{}},
					Extract:   ExternalName(),
					Namespace: "cool-ns",
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: ref,
				},
			},
		},
		"SuccessfulResolveInferredNamespace": {
			reason: "Resolve should be successful with namespace inferred from MR, when reference omits namespace",
			c: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					if key.Namespace == "from-ns" {
						meta.SetExternalName(obj.(metav1.Object), value)
						return nil
					}

					t.Errorf("Resolve did not infer to the MR namespace: %v", key)

					return errBoom
				},
			},
			from: &fake.Managed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-mr",
					Namespace: "from-ns",
				},
			},
			args: args{
				req: NamespacedResolutionRequest{
					Reference: nsOmittedRef,
					To:        To{Managed: &fake.Managed{}},
					Extract:   ExternalName(),
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: nsOmittedRef,
				},
			},
		},
		"SuccessfulResolveCrossNamespace": {
			reason: "Resolve should be successful when a namespace is given",
			c: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					if key.Namespace == "cool-ns" {
						meta.SetExternalName(obj.(metav1.Object), value)
						return nil
					}

					t.Errorf("Resolve did not infer to the other namespace: %v", key)

					return errBoom
				},
			},
			from: &fake.Managed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-mr",
					Namespace: "from-ns",
				},
			},
			args: args{
				req: NamespacedResolutionRequest{
					Reference: ref,
					To:        To{Managed: &fake.Managed{}},
					Extract:   ExternalName(),
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: ref,
				},
			},
		},
		"OptionalReference": {
			reason: "No error should be returned when the resolution policy is Optional",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Reference: optionalRef,
					To:        To{Managed: &fake.Managed{}},
					Extract:   func(resource.Managed) string { return "" },
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedReference: optionalRef,
				},
				err: nil,
			},
		},
		"ListError": {
			reason: "Should return errors encountered while listing potential referenced resources",
			c: &test.MockClient{
				MockList: test.NewMockListFn(errBoom),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{},
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{},
				err: errors.Wrap(errBoom, errListManaged),
			},
		},
		"NoMatches": {
			reason: "Should return an error when no managed resources match the selector",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{},
					To:       To{List: &FakeManagedList{}},
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{},
				err: errors.New(errNoMatches),
			},
		},
		"OptionalSelector": {
			reason: "No error should be returned when the resolution policy is Optional",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						Policy: &xpv1.Policy{Resolution: &optionalPolicy},
					},
					To: To{List: &FakeManagedList{}},
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{},
				err: nil,
			},
		},
		"SuccessfulSelect": {
			reason: "A managed resource with a matching controller reference should be selected and returned",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: controlled,
			args: args{
				req: NamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
					},
					To: To{List: &FakeManagedList{Items: []resource.Managed{
						&fake.Managed{}, // A resource that does not match.
						controlled,      // A resource with a matching controller reference.
					}}},
					Extract: ExternalName(),
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: &xpv1.NamespacedReference{Name: value},
				},
				err: nil,
			},
		},
		"SuccessfulSelectNamespaced": {
			reason: "Resolve should be successful when a namespace is given",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: controlled,
			args: args{
				req: NamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
					},
					To: To{List: &FakeManagedList{Items: []resource.Managed{
						&fake.Managed{}, // A resource that does not match.
						controlled,      // A resource with a matching controller reference.
					}}},
					Extract:   ExternalName(),
					Namespace: "cool-ns",
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: &xpv1.NamespacedReference{Name: value},
				},
				err: nil,
			},
		},
		"AlwaysResolveSelector": {
			reason: "Should not return early if the current value is non-zero, when the resolve policy is set to" +
				"Always",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: controlled,
			args: args{
				req: NamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
						Policy:             &xpv1.Policy{Resolve: &alwaysPolicy},
					},
					To: To{List: &FakeManagedList{Items: []resource.Managed{
						&fake.Managed{}, // A resource that does not match.
						controlled,      // A resource with a matching controller reference.
					}}},
					Extract:      ExternalName(),
					CurrentValue: "oldValue",
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: &xpv1.NamespacedReference{Name: value},
				},
				err: nil,
			},
		},
		"BothReferenceSelector": {
			reason: "When both Reference and Selector fields set and Policy is not set, the Reference must be resolved",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					meta.SetExternalName(obj.(metav1.Object), value)
					return nil
				}),
			},
			from: &fake.Managed{},
			args: args{
				req: NamespacedResolutionRequest{
					Reference: ref,
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
					},
					To:      To{Managed: &fake.Managed{}},
					Extract: ExternalName(),
				},
			},
			want: want{
				rsp: NamespacedResolutionResponse{
					ResolvedValue:     value,
					ResolvedReference: ref,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPINamespacedResolver(tc.c, tc.from)

			got, err := r.Resolve(tc.args.ctx, tc.args.req)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nControllersMustMatch(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.rsp, got); diff != "" {
				t.Errorf("\n%s\nControllersMustMatch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNamespacedResolveMultiple(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()
	value := "coolv"
	value2 := "cooler"
	ref := xpv1.NamespacedReference{Name: "cool", Namespace: "cool-ns"}
	nsOmittedRef := xpv1.NamespacedReference{Name: "cool"}
	optionalPolicy := xpv1.ResolutionPolicyOptional
	alwaysPolicy := xpv1.ResolvePolicyAlways
	optionalRef := xpv1.NamespacedReference{Name: "cool", Policy: &xpv1.Policy{Resolution: &optionalPolicy}}
	alwaysRef := xpv1.NamespacedReference{Name: "cool", Policy: &xpv1.Policy{Resolve: &alwaysPolicy}}

	controlled := &fake.Managed{}
	controlled.SetName(value)
	meta.SetExternalName(controlled, value)
	meta.AddControllerReference(controlled, meta.AsController(&xpv1.TypedReference{UID: types.UID("very-unique")}))

	controlled2 := &fake.Managed{}
	controlled2.SetName(value2)
	meta.SetExternalName(controlled2, value2)
	meta.AddControllerReference(controlled2, meta.AsController(&xpv1.TypedReference{UID: types.UID("very-unique")}))

	type args struct {
		ctx context.Context
		req MultiNamespacedResolutionRequest
	}

	type want struct {
		rsp MultiNamespacedResolutionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Reader
		from   resource.Managed
		args   args
		want   want
	}{
		"FromDeleted": {
			reason: "Should return early if the referencing managed resource was deleted",
			from:   &fake.Managed{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}},
			args: args{
				req: MultiNamespacedResolutionRequest{},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{},
				err: nil,
			},
		},
		"AlreadyResolved": {
			reason: "Should return early if the current value is non-zero",
			from:   &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{CurrentValues: []string{value}},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{ResolvedValues: []string{value}},
				err: nil,
			},
		},
		"AlwaysResolveReference": {
			reason: "Should not return early if the current value is non-zero, when the resolve policy is set to" +
				"Always",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					meta.SetExternalName(obj.(metav1.Object), value)
					return nil
				}),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References:    []xpv1.NamespacedReference{alwaysRef},
					To:            To{Managed: &fake.Managed{}},
					Extract:       ExternalName(),
					CurrentValues: []string{"oldValue"},
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{alwaysRef},
				},
				err: nil,
			},
		},
		"Unresolvable": {
			reason: "Should return early if neither a reference or selector were provided",
			from:   &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{},
			},
			want: want{
				err: nil,
			},
		},
		"GetError": {
			reason: "Should return errors encountered while getting the referenced resource",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{ref},
					To:         To{Managed: &fake.Managed{}},
					Extract:    ExternalName(),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetManaged),
			},
		},
		"ResolvedNoValue": {
			reason: "Should return an error if the extract function returns the empty string",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{ref},
					To:         To{Managed: &fake.Managed{}},
					Extract:    func(resource.Managed) string { return "" },
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{""},
					ResolvedReferences: []xpv1.NamespacedReference{ref},
				},
				err: errors.New(errNoValue),
			},
		},
		"SuccessfulResolve": {
			reason: "No error should be returned when the value is successfully extracted",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					meta.SetExternalName(obj.(metav1.Object), value)
					return nil
				}),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{ref},
					To:         To{Managed: &fake.Managed{}},
					Extract:    ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{ref},
				},
			},
		},
		"SuccessfulResolveNamespaced": {
			reason: "Resolve should be successful when a namespace is given",
			c: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					if key.Namespace == ref.Namespace {
						meta.SetExternalName(obj.(metav1.Object), value)
						return nil
					}

					t.Errorf("Resolve did not infer to the MR namespace: %v", key)

					return errBoom
				},
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{ref},
					To:         To{Managed: &fake.Managed{}},
					Extract:    ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{ref},
				},
			},
		},
		"SuccessfulResolveInferredNamespace": {
			reason: "Resolve should be successful when a namespace is given",
			c: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					if key.Namespace == "from-ns" {
						meta.SetExternalName(obj.(metav1.Object), value)
						return nil
					}

					t.Errorf("Resolve did not infer to the MR namespace: %v", key)

					return errBoom
				},
			},
			from: &fake.Managed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-mr",
					Namespace: "from-ns",
				},
			},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{nsOmittedRef},
					To:         To{Managed: &fake.Managed{}},
					Extract:    ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{nsOmittedRef},
				},
			},
		},
		"SuccessfulResolveCrossNamespace": {
			reason: "Resolve should be successful when a namespace is given",
			c: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					if key.Namespace == ref.Namespace {
						meta.SetExternalName(obj.(metav1.Object), value)
						return nil
					}

					t.Errorf("Resolve did not infer to the MR namespace: %v", key)

					return errBoom
				},
			},
			from: &fake.Managed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-mr",
					Namespace: "from-ns",
				},
			},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{ref},
					To:         To{Managed: &fake.Managed{}},
					Extract:    ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{ref},
				},
			},
		},
		"OptionalReference": {
			reason: "No error should be returned when the resolution policy is Optional",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{optionalRef},
					To:         To{Managed: &fake.Managed{}},
					Extract:    func(resource.Managed) string { return "" },
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{""},
					ResolvedReferences: []xpv1.NamespacedReference{optionalRef},
				},
				err: nil,
			},
		},
		"ListError": {
			reason: "Should return errors encountered while listing potential referenced resources",
			c: &test.MockClient{
				MockList: test.NewMockListFn(errBoom),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{},
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{},
				err: errors.Wrap(errBoom, errListManaged),
			},
		},
		"NoMatches": {
			reason: "Should return an error when no managed resources match the selector",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{},
					To:       To{List: &FakeManagedList{}},
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{},
				err: errors.New(errNoMatches),
			},
		},
		"OptionalSelector": {
			reason: "No error should be returned when the resolution policy is Optional",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						Policy: &xpv1.Policy{Resolution: &optionalPolicy},
					},
					To: To{List: &FakeManagedList{}},
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{},
				err: nil,
			},
		},
		"SuccessfulSelect": {
			reason: "A managed resource with a matching controller reference should be selected and returned",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: controlled,
			args: args{
				req: MultiNamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
					},
					To: To{List: &FakeManagedList{Items: []resource.Managed{
						&fake.Managed{}, // A resource that does not match.
						controlled,      // A resource with a matching controller reference.
					}}},
					Extract: ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{{Name: value}},
				},
				err: nil,
			},
		},
		"SuccessfulSelectNamespaced": {
			reason: "Resolve should be successful when a namespace is given",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: controlled,
			args: args{
				req: MultiNamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
					},
					To: To{List: &FakeManagedList{Items: []resource.Managed{
						&fake.Managed{}, // A resource that does not match.
						controlled,      // A resource with a matching controller reference.
					}}},
					Extract:   ExternalName(),
					Namespace: "cool-ns",
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{{Name: value}},
				},
				err: nil,
			},
		},
		"AlwaysResolveSelector": {
			reason: "Should not return early if the current value is non-zero, when the resolve policy is set to" +
				"Always",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: controlled,
			args: args{
				req: MultiNamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
						Policy:             &xpv1.Policy{Resolve: &alwaysPolicy},
					},
					To: To{List: &FakeManagedList{Items: []resource.Managed{
						&fake.Managed{}, // A resource that does not match.
						controlled,      // A resource with a matching controller reference.
					}}},
					Extract:       ExternalName(),
					CurrentValues: []string{"oldValue"},
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{{Name: value}},
				},
				err: nil,
			},
		},
		"BothReferenceSelector": {
			reason: "When both Reference and Selector fields set and Policy is not set, the Reference must be resolved",
			c: &test.MockClient{
				MockList: test.NewMockListFn(errors.New("unexpected call to List when resolving Refs only")),
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					meta.SetExternalName(obj.(metav1.Object), value)
					return nil
				}),
			},
			from: &fake.Managed{},
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{ref},
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
					},
					To:      To{Managed: &fake.Managed{}},
					Extract: ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value},
					ResolvedReferences: []xpv1.NamespacedReference{ref},
				},
			},
		},
		"SelectorOrderOutput": {
			reason: "Resolved values should be ordered when resolving a selector",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
			},
			from: controlled,
			args: args{
				req: MultiNamespacedResolutionRequest{
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
					},
					To: To{List: &FakeManagedList{
						Items: []resource.Managed{
							&fake.Managed{}, // A resource that does not match.
							controlled,      // A resource with a matching controller reference.
							&fake.Managed{}, // A resource that does not match.
							controlled2,     // A resource with a matching controller reference.
						},
					}},
					Extract: ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{value2, value},
					ResolvedReferences: []xpv1.NamespacedReference{{Name: value2}, {Name: value}},
				},
				err: nil,
			},
		},
		"NoSelectorOnlyRefs": {
			reason: "Refs should not be re-ordered when selector is omitted",
			c: &test.MockClient{
				MockList: test.NewMockListFn(errors.New("unexpected call to List when resolving Refs only")),
				MockGet: func(_ context.Context, objKey client.ObjectKey, obj client.Object) error {
					if !strings.HasPrefix(objKey.Name, testResourceNamePrefix) {
						return errors.New("test resource not found")
					}
					val := strings.Replace(objKey.Name, testResourceNamePrefix, testValuePrefix, 1)
					meta.SetExternalName(obj.(metav1.Object), val)
					return nil
				},
			},
			from: controlled,
			args: args{
				req: MultiNamespacedResolutionRequest{
					References: []xpv1.NamespacedReference{nsTestRefs[2], nsTestRefs[3], nsTestRefs[0], nsTestRefs[1]},
					To: To{
						Managed: &fake.Managed{},
					},
					Extract: ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					ResolvedValues:     []string{nsTestValues[2], nsTestValues[3], nsTestValues[0], nsTestValues[1]},
					ResolvedReferences: []xpv1.NamespacedReference{nsTestRefs[2], nsTestRefs[3], nsTestRefs[0], nsTestRefs[1]},
				},
				err: nil,
			},
		},
		"AlwaysResolveSelector_NewValuesOrdered": {
			reason: "Must resolve new matches and reorder resolved values & refs, when Selector policy is Always and have existing refs and values",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil),
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					meta.SetExternalName(obj.(metav1.Object), value)
					return nil
				}),
			},
			from: controlled,
			args: args{
				req: MultiNamespacedResolutionRequest{
					CurrentValues: []string{nsTestValues[1], nsTestValues[4]},
					References:    []xpv1.NamespacedReference{nsTestRefs[1], nsTestRefs[4]},
					Selector: &xpv1.NamespacedSelector{
						MatchControllerRef: func() *bool { t := true; return &t }(),
						Policy:             &xpv1.Policy{Resolve: &alwaysPolicy},
					},
					To: To{
						Managed: &fake.Managed{},
						List: &FakeManagedList{
							// List result is not ordered
							Items: []resource.Managed{
								&fake.Managed{},     // A resource that does not match.
								nsTestControlled[2], // A resource with a matching controller reference.
								&fake.Managed{},     // A resource that does not match.
								nsTestControlled[4], // A resource with a matching controller reference.
								nsTestControlled[1], // A resource with a matching controller reference and newly introduced
							},
						},
					},
					Extract: ExternalName(),
				},
			},
			want: want{
				rsp: MultiNamespacedResolutionResponse{
					// expect ordered resolved values
					ResolvedValues:     []string{nsTestValues[1], nsTestValues[2], nsTestValues[4]},
					ResolvedReferences: []xpv1.NamespacedReference{nsTestRefs[1], nsTestRefs[2], nsTestRefs[4]},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPINamespacedResolver(tc.c, tc.from)

			got, err := r.ResolveMultiple(tc.args.ctx, tc.args.req)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nControllersMustMatch(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.rsp, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nControllersMustMatch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNamespacedControllersMustMatch(t *testing.T) {
	cases := map[string]struct {
		s    *xpv1.NamespacedSelector
		want bool
	}{
		"NilSelector": {
			s:    nil,
			want: false,
		},
		"NilMatchControllerRef": {
			s:    &xpv1.NamespacedSelector{},
			want: false,
		},
		"False": {
			s:    &xpv1.NamespacedSelector{MatchControllerRef: func() *bool { f := false; return &f }()},
			want: false,
		},
		"True": {
			s:    &xpv1.NamespacedSelector{MatchControllerRef: func() *bool { t := true; return &t }()},
			want: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ControllersMustMatchNamespaced(tc.s)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ControllersMustMatch(...): -want, +got:\n%s", diff)
			}
		})
	}
}
