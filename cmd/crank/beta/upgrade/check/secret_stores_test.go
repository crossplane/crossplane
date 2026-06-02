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

package check

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	secretsv1alpha1 "github.com/crossplane/crossplane/apis/secrets/v1alpha1"
)

func TestContainerHasEnabledFlag(t *testing.T) {
	const flag = flagEnableExternalSecretStores

	cases := map[string]struct {
		reason string
		args   []string
		want   bool
	}{
		"Absent":             {reason: "The flag isn't present.", args: []string{"--other"}, want: false},
		"Empty":              {reason: "No args at all.", args: nil, want: false},
		"Standalone":         {reason: "A bare flag with no value enables it.", args: []string{flag}, want: true},
		"FollowedByFalse":    {reason: "A bare flag whose next arg is \"false\" is disabled.", args: []string{flag, "false"}, want: false},
		"FollowedByFalseCap": {reason: "Disabling value is matched case-insensitively.", args: []string{flag, "False"}, want: false},
		"FollowedByFlag":     {reason: "A following flag is not a value, so the flag stays enabled.", args: []string{flag, "--metrics"}, want: true},
		"FollowedByValue":    {reason: "A following non-false value leaves the flag enabled.", args: []string{flag, "something"}, want: true},
		"EqualsTrue":         {reason: "flag=true enables it.", args: []string{flag + "=true"}, want: true},
		"EqualsFalse":        {reason: "flag=false disables it.", args: []string{flag + "=false"}, want: false},
		"EqualsFalseCap":     {reason: "flag=False is matched case-insensitively.", args: []string{flag + "=False"}, want: false},
		"EqualsOtherValue":   {reason: "flag=<non-false> enables it.", args: []string{flag + "=1"}, want: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := containerHasEnabledFlag(tc.args, flag)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\ncontainerHasEnabledFlag(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsAutoCreatedDefaultStoreConfig(t *testing.T) {
	cases := map[string]struct {
		reason string
		sc     secretsv1alpha1.StoreConfig
		want   bool
	}{
		"CleanDefault": {
			reason: "The bare \"default\" StoreConfig the init controller creates should be detected as such.",
			sc:     secretsv1alpha1.StoreConfig{ObjectMeta: metav1.ObjectMeta{Name: defaultStoreConfigName}},
			want:   true,
		},
		"ExplicitKubernetesType": {
			reason: "An explicit Kubernetes type still matches the auto-created shape.",
			sc: secretsv1alpha1.StoreConfig{
				ObjectMeta: metav1.ObjectMeta{Name: defaultStoreConfigName},
				Spec: secretsv1alpha1.StoreConfigSpec{
					SecretStoreConfig: xpv1.SecretStoreConfig{Type: ptr.To(xpv1.SecretStoreKubernetes)},
				},
			},
			want: true,
		},
		"WrongName": {
			reason: "Any name other than \"default\" is user-created.",
			sc:     secretsv1alpha1.StoreConfig{ObjectMeta: metav1.ObjectMeta{Name: "custom"}},
			want:   false,
		},
		"NonKubernetesType": {
			reason: "A non-Kubernetes store type signals user intent.",
			sc: secretsv1alpha1.StoreConfig{
				ObjectMeta: metav1.ObjectMeta{Name: defaultStoreConfigName},
				Spec: secretsv1alpha1.StoreConfigSpec{
					SecretStoreConfig: xpv1.SecretStoreConfig{Type: ptr.To(xpv1.SecretStoreType("Vault"))},
				},
			},
			want: false,
		},
		"KubernetesConfigSet": {
			reason: "A populated Kubernetes config disqualifies the auto-created shape.",
			sc: secretsv1alpha1.StoreConfig{
				ObjectMeta: metav1.ObjectMeta{Name: defaultStoreConfigName},
				Spec: secretsv1alpha1.StoreConfigSpec{
					SecretStoreConfig: xpv1.SecretStoreConfig{Type: ptr.To(xpv1.SecretStoreKubernetes), Kubernetes: &xpv1.KubernetesSecretStoreConfig{}},
				},
			},
			want: false,
		},
		"PluginSet": {
			reason: "A populated Plugin config disqualifies the auto-created shape.",
			sc: secretsv1alpha1.StoreConfig{
				ObjectMeta: metav1.ObjectMeta{Name: defaultStoreConfigName},
				Spec: secretsv1alpha1.StoreConfigSpec{
					SecretStoreConfig: xpv1.SecretStoreConfig{Plugin: &xpv1.PluginStoreConfig{}},
				},
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sc := tc.sc
			got := isAutoCreatedDefaultStoreConfig(&sc)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nisAutoCreatedDefaultStoreConfig(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// mrInstance builds an unstructured managed resource with a user-set
// spec.publishConnectionDetailsTo, the shape checkManagedResources flags.
func mrInstance(apiVersion, kind, name string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"publishConnectionDetailsTo": map[string]any{"name": "secret"}},
	}}
}

// essData is the declarative input for one Run scenario. Zero-value fields mean
// "the corresponding List returns nothing". instances is keyed by list kind
// (e.g. "XThingList", "BucketList"); instErr is keyed the same way so a case can
// fail a single instance List without disturbing the others.
type essData struct {
	deploys      []appsv1.Deployment
	storeConfigs []secretsv1alpha1.StoreConfig
	comps        []apiextensionsv1.Composition
	xrds         []apiextensionsv1.CompositeResourceDefinition
	crds         []extv1.CustomResourceDefinition
	instances    map[string][]unstructured.Unstructured

	deployErr      error
	storeConfigErr error
	compErr        error
	xrdErr         error
	crdErr         error
	instErr        map[string]error
}

func essClient(d essData) client.Client {
	return &test.MockClient{
		MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
			switch l := list.(type) {
			case *appsv1.DeploymentList:
				if d.deployErr != nil {
					return d.deployErr
				}
				l.Items = d.deploys
			case *secretsv1alpha1.StoreConfigList:
				if d.storeConfigErr != nil {
					return d.storeConfigErr
				}
				l.Items = d.storeConfigs
			case *apiextensionsv1.CompositionList:
				if d.compErr != nil {
					return d.compErr
				}
				l.Items = d.comps
			case *apiextensionsv1.CompositeResourceDefinitionList:
				if d.xrdErr != nil {
					return d.xrdErr
				}
				l.Items = d.xrds
			case *extv1.CustomResourceDefinitionList:
				if d.crdErr != nil {
					return d.crdErr
				}
				l.Items = d.crds
			case *unstructured.UnstructuredList:
				kind := l.GetObjectKind().GroupVersionKind().Kind
				if err := d.instErr[kind]; err != nil {
					return err
				}
				l.Items = d.instances[kind]
			}
			return nil
		},
	}
}

func TestExternalSecretStoresRun(t *testing.T) {
	type want struct {
		findings []Finding
		err      error
	}
	cases := map[string]struct {
		reason      string
		selector    string // when empty, this test will set it to the CLI flag default "app=crossplane"
		skipMR      bool
		concurrency int // when zero, this test will set it to 10
		data        essData
		want        want
	}{
		"Clean": {
			reason: "An empty control plane (including an empty managed resource scan) produces no findings.",
			data:   essData{},
			want:   want{findings: nil},
		},
		"BadSelector": {
			reason:   "An unparseable Crossplane label selector is a hard error.",
			selector: "!!!bad",
			skipMR:   true,
			data:     essData{},
			want:     want{err: cmpopts.AnyError},
		},
		"ListDeploymentsError": {
			reason: "A failure listing Crossplane Deployments is a hard error.",
			skipMR: true,
			data:   essData{deployErr: errBoom},
			want:   want{err: cmpopts.AnyError},
		},
		"DeploymentFlagEnabled": {
			reason: "A Crossplane Deployment running with --enable-external-secret-stores is flagged.",
			skipMR: true,
			data: essData{deploys: []appsv1.Deployment{{
				ObjectMeta: metav1.ObjectMeta{Name: "crossplane", Namespace: "crossplane-system"},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "crossplane", Args: []string{"--enable-external-secret-stores"}}},
						},
					},
				},
			}}},
			want: want{findings: []Finding{{
				Resource:  ResourceRef{Group: appsv1.SchemeGroupVersion.Group, Kind: "Deployment", Namespace: "crossplane-system", Name: "crossplane"},
				FieldPath: ".spec.template.spec.containers[0].args",
			}}},
		},
		"DeploymentFlagDisabled": {
			reason: "A Deployment that explicitly disables the flag is not flagged.",
			skipMR: true,
			data: essData{deploys: []appsv1.Deployment{{
				ObjectMeta: metav1.ObjectMeta{Name: "crossplane", Namespace: "crossplane-system"},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "crossplane", Args: []string{"--enable-external-secret-stores=false"}}},
						},
					},
				},
			}}},
			want: want{findings: nil},
		},
		"NonDefaultStoreConfig": {
			reason: "A user-created StoreConfig is flagged.",
			skipMR: true,
			data:   essData{storeConfigs: []secretsv1alpha1.StoreConfig{{ObjectMeta: metav1.ObjectMeta{Name: "custom"}}}},
			want:   want{findings: []Finding{{Resource: ResourceRef{Group: secretsv1alpha1.Group, Kind: secretsv1alpha1.StoreConfigKind, Name: "custom"}}}},
		},
		"DefaultStoreConfigIgnored": {
			reason: "The auto-created \"default\" StoreConfig is not flagged.",
			skipMR: true,
			data:   essData{storeConfigs: []secretsv1alpha1.StoreConfig{{ObjectMeta: metav1.ObjectMeta{Name: defaultStoreConfigName}}}},
			want:   want{findings: nil},
		},
		"ListStoreConfigsError": {
			reason: "A failure listing StoreConfigs is surfaced as an error.",
			skipMR: true,
			data:   essData{storeConfigErr: errBoom},
			want:   want{err: cmpopts.AnyError},
		},
		"CompositionStoreConfigRef": {
			reason: "A Composition whose publishConnectionDetailsWithStoreConfigRef.name differs from \"default\" is flagged.",
			skipMR: true,
			data: essData{comps: []apiextensionsv1.Composition{{
				ObjectMeta: metav1.ObjectMeta{Name: "comp"},
				Spec: apiextensionsv1.CompositionSpec{
					PublishConnectionDetailsWithStoreConfigRef: &apiextensionsv1.StoreConfigReference{Name: "custom"},
				},
			}}},
			want: want{findings: []Finding{{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "comp"}, FieldPath: ".spec.publishConnectionDetailsWithStoreConfigRef"}}},
		},
		"CompositionDefaultStoreConfigRefIgnored": {
			reason: "The apiserver-injected \"default\" store-config ref is not flagged.",
			skipMR: true,
			data: essData{comps: []apiextensionsv1.Composition{{
				ObjectMeta: metav1.ObjectMeta{Name: "comp"},
				Spec: apiextensionsv1.CompositionSpec{
					PublishConnectionDetailsWithStoreConfigRef: &apiextensionsv1.StoreConfigReference{Name: defaultStoreConfigName},
				},
			}}},
			want: want{findings: nil},
		},
		"XRUserOverrideFlaggedUIDFiltered": {
			reason: "An XR with a user-set publishConnectionDetailsTo is flagged; the controller's auto-injected entry (name == metadata.uid) is filtered out.",
			skipMR: true,
			data: essData{
				xrds: []apiextensionsv1.CompositeResourceDefinition{xrd("example.org", "XThing", "v1", "")},
				instances: map[string][]unstructured.Unstructured{
					"XThingList": {
						// publishConnectionDetailsTo.name == metadata.uid, so skipped
						{Object: map[string]any{
							"apiVersion": "example.org/v1",
							"kind":       "XThing",
							"metadata":   map[string]any{"name": "xr-auto", "uid": "uid-1"},
							"spec":       map[string]any{"publishConnectionDetailsTo": map[string]any{"name": "uid-1"}},
						}},
						// user override, flagged
						{Object: map[string]any{
							"apiVersion": "example.org/v1",
							"kind":       "XThing",
							"metadata":   map[string]any{"name": "xr-user", "uid": "uid-2"},
							"spec":       map[string]any{"publishConnectionDetailsTo": map[string]any{"name": "custom"}},
						}},
					},
				},
			},
			want: want{findings: []Finding{{Resource: ResourceRef{Group: "example.org", Kind: "XThing", Name: "xr-user"}, FieldPath: ".spec.publishConnectionDetailsTo"}}},
		},
		"ClaimFlagged": {
			reason: "A namespaced Claim with publishConnectionDetailsTo is flagged with no UID filtering.",
			skipMR: true,
			data: essData{
				xrds: []apiextensionsv1.CompositeResourceDefinition{xrd("example.org", "XThing", "v1", "Thing")},
				instances: map[string][]unstructured.Unstructured{
					"ThingList": {{Object: map[string]any{
						"apiVersion": "example.org/v1",
						"kind":       "Thing",
						"metadata":   map[string]any{"name": "claim-1", "namespace": "team-a"},
						"spec":       map[string]any{"publishConnectionDetailsTo": map[string]any{"name": "anything"}},
					}}},
				},
			},
			want: want{findings: []Finding{{Resource: ResourceRef{Group: "example.org", Kind: "Thing", Namespace: "team-a", Name: "claim-1"}, FieldPath: ".spec.publishConnectionDetailsTo"}}},
		},
		"XRInstanceListError": {
			reason: "A failure listing XR instances during the XR/Claim scan is a hard error.",
			skipMR: true,
			data: essData{
				xrds:    []apiextensionsv1.CompositeResourceDefinition{xrd("example.org", "XThing", "v1", "")},
				instErr: map[string]error{"XThingList": errBoom},
			},
			want: want{err: cmpopts.AnyError},
		},
		"DiscoverXRTypesError": {
			reason: "A failure listing XRDs while discovering XR/Claim types is a hard error.",
			skipMR: true,
			data:   essData{xrdErr: errBoom},
			want:   want{err: cmpopts.AnyError},
		},
		"DiscoverManagedResourcesError": {
			reason: "A failure listing CRDs while discovering managed resource types is a hard error.",
			data:   essData{crdErr: errBoom},
			want:   want{err: cmpopts.AnyError},
		},
		"ManagedResourceFlagged": {
			reason: "A managed resource with a user-set publishConnectionDetailsTo is flagged when the MR scan runs.",
			data: essData{
				crds: []extv1.CustomResourceDefinition{managedCRD("aws.example.org", "Bucket", "v1", false)},
				instances: map[string][]unstructured.Unstructured{
					"BucketList": {{Object: map[string]any{
						"apiVersion": "aws.example.org/v1",
						"kind":       "Bucket",
						"metadata":   map[string]any{"name": "my-bucket"},
						"spec":       map[string]any{"publishConnectionDetailsTo": map[string]any{"name": "secret"}},
					}}},
				},
			},
			want: want{findings: []Finding{{Resource: ResourceRef{Group: "aws.example.org", Kind: "Bucket", Name: "my-bucket"}, FieldPath: ".spec.publishConnectionDetailsTo"}}},
		},
		"SkipManagedResources": {
			reason: "With --skip-managed-resources the MR scan is skipped, so a flaggable managed resource produces no finding.",
			skipMR: true,
			data: essData{
				crds: []extv1.CustomResourceDefinition{managedCRD("aws.example.org", "Bucket", "v1", false)},
				instances: map[string][]unstructured.Unstructured{
					"BucketList": {{Object: map[string]any{
						"apiVersion": "aws.example.org/v1",
						"kind":       "Bucket",
						"metadata":   map[string]any{"name": "my-bucket"},
						"spec":       map[string]any{"publishConnectionDetailsTo": map[string]any{"name": "secret"}},
					}}},
				},
			},
			want: want{findings: nil},
		},
		"ManagedResourceListErrorAggregated": {
			reason: "When one managed resource type's List fails, its error is surfaced while findings from healthy types still come back - one flaky CRD doesn't drop the rest.",
			data: essData{
				crds: []extv1.CustomResourceDefinition{
					managedCRD("aws.example.org", "Bucket", "v1", false),
					managedCRD("other.example.org", "Queue", "v1", false),
				},
				instErr: map[string]error{"BucketList": errBoom}, // only return an error when listing Buckets
				instances: map[string][]unstructured.Unstructured{
					"QueueList": {{Object: map[string]any{
						"apiVersion": "other.example.org/v1",
						"kind":       "Queue",
						"metadata":   map[string]any{"name": "my-queue"},
						"spec":       map[string]any{"publishConnectionDetailsTo": map[string]any{"name": "secret"}},
					}}},
				},
			},
			want: want{
				findings: []Finding{{Resource: ResourceRef{Group: "other.example.org", Kind: "Queue", Name: "my-queue"}, FieldPath: ".spec.publishConnectionDetailsTo"}},
				err:      cmpopts.AnyError,
			},
		},
		"ManagedResourcesExceedConcurrencyLimit": {
			reason:      "With more managed resource types than the concurrency limit, the bounded scan must process every type without deadlocking or dropping findings. Findings come back in type-discovery order because each type writes its own result slot.",
			concurrency: 2, // small concurency limit of 2, less than the number of MR types we need to check
			data: essData{
				crds: []extv1.CustomResourceDefinition{
					managedCRD("a.example.org", "Aa", "v1", false),
					managedCRD("b.example.org", "Bb", "v1", false),
					managedCRD("c.example.org", "Cc", "v1", false),
					managedCRD("d.example.org", "Dd", "v1", false),
					managedCRD("e.example.org", "Ee", "v1", false),
				},
				instances: map[string][]unstructured.Unstructured{
					"AaList": {mrInstance("a.example.org/v1", "Aa", "aa")},
					"BbList": {mrInstance("b.example.org/v1", "Bb", "bb")},
					"CcList": {mrInstance("c.example.org/v1", "Cc", "cc")},
					"DdList": {mrInstance("d.example.org/v1", "Dd", "dd")},
					"EeList": {mrInstance("e.example.org/v1", "Ee", "ee")},
				},
			},
			want: want{findings: []Finding{
				{Resource: ResourceRef{Group: "a.example.org", Kind: "Aa", Name: "aa"}, FieldPath: ".spec.publishConnectionDetailsTo"},
				{Resource: ResourceRef{Group: "b.example.org", Kind: "Bb", Name: "bb"}, FieldPath: ".spec.publishConnectionDetailsTo"},
				{Resource: ResourceRef{Group: "c.example.org", Kind: "Cc", Name: "cc"}, FieldPath: ".spec.publishConnectionDetailsTo"},
				{Resource: ResourceRef{Group: "d.example.org", Kind: "Dd", Name: "dd"}, FieldPath: ".spec.publishConnectionDetailsTo"},
				{Resource: ResourceRef{Group: "e.example.org", Kind: "Ee", Name: "ee"}, FieldPath: ".spec.publishConnectionDetailsTo"},
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			selector := tc.selector
			if selector == "" {
				selector = "app=crossplane"
			}
			concurrency := tc.concurrency
			if concurrency == 0 {
				concurrency = 10
			}
			c := &ExternalSecretStores{
				Client:               essClient(tc.data),
				CrossplaneNamespace:  "crossplane-system",
				Selector:             selector,
				SkipManagedResources: tc.skipMR,
				Concurrency:          concurrency,
			}
			got, err := c.Run(context.Background())
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.findings, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRun(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCheckXRsAndClaimsNamespaceScope(t *testing.T) {
	// checkXRsAndClaims lists cluster-scoped XRs across the whole cluster but
	// scopes namespaced Claim lookups to ClaimNamespace (empty means all
	// namespaces). We capture the namespace ListInstances applies per list kind
	// to prove that selection. The single XRD below yields both an XThing (XR)
	// and a Thing (Claim) type, so one call exercises both branches.
	cases := map[string]struct {
		reason         string
		claimNamespace string
		want           map[string]string // map of "list kind": "namespace used for list call"
	}{
		"ScopedToClaimNamespace": {
			reason:         "A set ClaimNamespace scopes the Claim list but not the cluster-scoped XR list.",
			claimNamespace: "team-a",
			want:           map[string]string{"XThingList": "", "ThingList": "team-a"},
		},
		"AllNamespacesWhenEmpty": {
			reason:         "An empty ClaimNamespace lists Claims across all namespaces, so neither list is namespace-scoped.",
			claimNamespace: "",
			want:           map[string]string{"XThingList": "", "ThingList": ""},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotNS := map[string]string{}
			c := &ExternalSecretStores{
				ClaimNamespace: tc.claimNamespace,
				Client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
						switch l := list.(type) {
						case *apiextensionsv1.CompositeResourceDefinitionList:
							l.Items = []apiextensionsv1.CompositeResourceDefinition{xrd("example.org", "XThing", "v1", "Thing")}
						case *unstructured.UnstructuredList:
							ns := ""
							for _, o := range opts {
								if in, ok := o.(client.InNamespace); ok {
									ns = string(in)
								}
							}
							gotNS[l.GetObjectKind().GroupVersionKind().Kind] = ns
						}
						return nil
					},
				},
			}

			if _, err := c.checkXRsAndClaims(context.Background()); err != nil {
				t.Fatalf("checkXRsAndClaims() unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, gotNS); diff != "" {
				t.Errorf("\n%s\ncheckXRsAndClaims() namespace per list kind: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
