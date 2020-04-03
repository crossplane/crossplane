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

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

const (
	namespace       = "coolNamespace"
	name            = "coolAR"
	uid             = types.UID("definitely-a-uuid")
	resourceVersion = "coolVersion"
)

// url.Parse returns a slightly different error string in Go 1.14 than in prior versions.
func urlParseError(s string) error {
	_, err := url.Parse(s)
	return err
}

var (
	errorBoom  = errors.New("boom")
	errJSON    = errors.New("unexpected end of JSON input")
	objectMeta = metav1.ObjectMeta{
		Namespace:  namespace,
		Name:       name,
		UID:        uid,
		Finalizers: []string{},
	}
	ctx = context.Background()

	target = &v1alpha1.KubernetesTarget{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolTarget"},
		Spec: runtimev1alpha1.TargetSpec{
			WriteConnectionSecretToReference: &runtimev1alpha1.LocalSecretReference{Name: secret.GetName()},
		},
	}

	targetRef = &v1alpha1.KubernetesTargetReference{Name: target.GetName()}

	apiServerURL, _ = url.Parse("https://example.org")
	malformedURL    = ":wat:"

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "coolSecret",
			Namespace: namespace,
			Annotations: map[string]string{
				RemoteControllerNamespace: objectMeta.GetNamespace(),
				RemoteControllerName:      objectMeta.GetName(),
				RemoteControllerUID:       string(objectMeta.GetUID()),
			},
		},
		Data: map[string][]byte{
			runtimev1alpha1.ResourceCredentialsSecretEndpointKey:   []byte(apiServerURL.String()),
			runtimev1alpha1.ResourceCredentialsSecretUserKey:       []byte("user"),
			runtimev1alpha1.ResourceCredentialsSecretPasswordKey:   []byte("password"),
			runtimev1alpha1.ResourceCredentialsSecretCAKey:         []byte("secretCA"),
			runtimev1alpha1.ResourceCredentialsSecretClientCertKey: []byte("clientCert"),
			runtimev1alpha1.ResourceCredentialsSecretClientKeyKey:  []byte("clientKey"),
			runtimev1alpha1.ResourceCredentialsSecretTokenKey:      []byte("token"),
		},
	}

	existingSecret = func() *corev1.Secret {
		s := secret.DeepCopy()
		s.Data["extrafield"] = []byte("somuchmore!")
		return s
	}()

	secretLocalObjectRef = corev1.LocalObjectReference{Name: secret.GetName()}

	secretWithExplicitType = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basicAuthSecret",
			Namespace: namespace,
			Annotations: map[string]string{
				RemoteControllerNamespace: objectMeta.GetNamespace(),
				RemoteControllerName:      objectMeta.GetName(),
				RemoteControllerUID:       string(objectMeta.GetUID()),
			},
		},
		Data: map[string][]byte{
			runtimev1alpha1.ResourceCredentialsSecretUserKey:     []byte("user"),
			runtimev1alpha1.ResourceCredentialsSecretPasswordKey: []byte("password"),
		},
		Type: corev1.SecretTypeBasicAuth,
	}

	serviceWithoutNamespace = &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		// Note we purposefully omit the namespace here in order to test our
		// namespace defaulting logic.
		ObjectMeta: metav1.ObjectMeta{
			Name: "coolService",
			Annotations: map[string]string{
				RemoteControllerNamespace: objectMeta.GetNamespace(),
				RemoteControllerName:      objectMeta.GetName(),
				RemoteControllerUID:       string(objectMeta.GetUID()),
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{Hostname: "coolservice.crossplane.io"},
				},
			},
		},
	}

	service = func() *corev1.Service {
		s := serviceWithoutNamespace.DeepCopy()
		s.SetNamespace(namespace)
		return s
	}()

	existingService = func() *corev1.Service {
		s := service.DeepCopy()
		s.Spec.Type = corev1.ServiceTypeClusterIP
		return s
	}()

	remoteStatus = func() *v1alpha1.RemoteStatus {
		raw, _ := json.Marshal(serviceWithoutNamespace.Status)
		return &v1alpha1.RemoteStatus{Raw: json.RawMessage(raw)}
	}()

	deleteTime = time.Now()
)

func template(s *corev1.Service) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = scheme.Convert(s, u, nil)
	return u
}

type kubeARModifier func(*v1alpha1.KubernetesApplicationResource)

func withFinalizers(f ...string) kubeARModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) { r.ObjectMeta.Finalizers = f }
}

func withConditions(c ...runtimev1alpha1.Condition) kubeARModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) { r.Status.SetConditions(c...) }
}

func withState(s v1alpha1.KubernetesApplicationResourceState) kubeARModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) { r.Status.State = s }
}

func withRemoteStatus(s *v1alpha1.RemoteStatus) kubeARModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) { r.Status.Remote = s }
}

func withDeletionTimestamp(t time.Time) kubeARModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) {
		r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t}
	}
}

func withTarget(name string) kubeARModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) {
		r.Spec.Target = &v1alpha1.KubernetesTargetReference{Name: name}
	}
}

func withSecrets(s ...corev1.LocalObjectReference) kubeARModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) {
		r.Spec.Secrets = s
	}
}

func withTemplate(t *unstructured.Unstructured) kubeARModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) {
		raw, _ := json.Marshal(t)
		r.Spec.Template = runtime.RawExtension{Raw: raw}
	}
}

func kubeAR(rm ...kubeARModifier) *v1alpha1.KubernetesApplicationResource {
	r := &v1alpha1.KubernetesApplicationResource{ObjectMeta: objectMeta}

	for _, m := range rm {
		m(r)
	}

	return r
}

func TestCreatePredicate(t *testing.T) {
	cases := []struct {
		name  string
		event event.CreateEvent
		want  bool
	}{
		{
			name: "ScheduledCluster",
			event: event.CreateEvent{
				Object: &v1alpha1.KubernetesApplicationResource{
					Spec: v1alpha1.KubernetesApplicationResourceSpec{
						Target: targetRef,
					},
				},
			},
			want: true,
		},
		{
			name: "UnscheduledCluster",
			event: event.CreateEvent{
				Object: &v1alpha1.KubernetesApplicationResource{},
			},
			want: false,
		},
		{
			name: "NotAKubernetesApplicationResource",
			event: event.CreateEvent{
				Object: &v1alpha1.KubernetesApplication{},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CreatePredicate(tc.event)
			if got != tc.want {
				t.Errorf("CreatePredicate(...): got %v, want %v", got, tc.want)
			}
		})
	}
}
func TestUpdatePredicate(t *testing.T) {
	cases := []struct {
		name  string
		event event.UpdateEvent
		want  bool
	}{
		{
			name: "ScheduledCluster",
			event: event.UpdateEvent{
				ObjectNew: &v1alpha1.KubernetesApplicationResource{
					Spec: v1alpha1.KubernetesApplicationResourceSpec{
						Target: targetRef,
					},
				},
			},
			want: true,
		},
		{
			name: "UnscheduledCluster",
			event: event.UpdateEvent{
				ObjectNew: &v1alpha1.KubernetesApplicationResource{},
			},
			want: false,
		},
		{
			name: "NotAKubernetesApplicationResource",
			event: event.UpdateEvent{
				ObjectNew: &v1alpha1.KubernetesApplication{},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := UpdatePredicate(tc.event)
			if got != tc.want {
				t.Errorf("UpdatePredicate(...): got %v, want %v", got, tc.want)
			}
		})
	}
}

type mockSyncUnstructuredFn func(ctx context.Context, template *unstructured.Unstructured) (*v1alpha1.RemoteStatus, error)

func newMockSyncUnstructuredFn(s *v1alpha1.RemoteStatus, err error) mockSyncUnstructuredFn {
	return func(_ context.Context, _ *unstructured.Unstructured) (*v1alpha1.RemoteStatus, error) {
		return s, err
	}
}

type mockDeleteUnstructuredFn func(ctx context.Context, template *unstructured.Unstructured) error

func newMockDeleteUnstructuredFn(err error) mockDeleteUnstructuredFn {
	return func(_ context.Context, _ *unstructured.Unstructured) error {
		return err
	}
}

type mockUnstructuredClient struct {
	mockSync   mockSyncUnstructuredFn
	mockDelete mockDeleteUnstructuredFn
}

func (m *mockUnstructuredClient) sync(ctx context.Context, template *unstructured.Unstructured) (*v1alpha1.RemoteStatus, error) {
	return m.mockSync(ctx, template)
}

func (m *mockUnstructuredClient) delete(ctx context.Context, template *unstructured.Unstructured) error {
	return m.mockDelete(ctx, template)
}

type mockSyncSecretFn func(ctx context.Context, template *corev1.Secret) error

func newMockSyncSecretFn(err error) mockSyncSecretFn {
	return func(ctx context.Context, template *corev1.Secret) error { return err }
}

type mockDeleteSecretFn func(ctx context.Context, template *corev1.Secret) error

func newMockDeleteSecretFn(err error) mockDeleteSecretFn {
	return func(ctx context.Context, template *corev1.Secret) error { return err }
}

type mockSecretClient struct {
	mockSync   mockSyncSecretFn
	mockDelete mockDeleteSecretFn
}

func (m *mockSecretClient) sync(ctx context.Context, template *corev1.Secret) error {
	return m.mockSync(ctx, template)
}

func (m *mockSecretClient) delete(ctx context.Context, template *corev1.Secret) error {
	return m.mockDelete(ctx, template)
}

func TestSync(t *testing.T) {
	cases := []struct {
		name      string
		syncer    syncer
		ar        *v1alpha1.KubernetesApplicationResource
		secrets   []corev1.Secret
		wantState v1alpha1.KubernetesApplicationResourceState
		wantErr   error
	}{
		{
			name: "Successful",
			syncer: &remoteCluster{
				unstructured: &mockUnstructuredClient{
					mockSync: func(_ context.Context, got *unstructured.Unstructured) (*v1alpha1.RemoteStatus, error) {
						want := template(service)
						want.SetNamespace(namespace)
						want.SetAnnotations(map[string]string{
							RemoteControllerNamespace: objectMeta.GetNamespace(),
							RemoteControllerName:      objectMeta.GetName(),
							RemoteControllerUID:       string(objectMeta.GetUID()),
						})
						if diff := cmp.Diff(want, got); diff != "" {
							return nil, errors.Errorf("mockSync: -want, +got: %s", diff)
						}

						return remoteStatus, nil
					},
				},
				secret: &mockSecretClient{
					mockSync: func(_ context.Context, got *corev1.Secret) error {
						want := secret.DeepCopy()
						want.SetName(fmt.Sprintf("%s-%s", objectMeta.GetName(), secret.GetName()))
						want.SetNamespace(namespace)
						want.SetAnnotations(map[string]string{
							RemoteControllerNamespace: objectMeta.GetNamespace(),
							RemoteControllerName:      objectMeta.GetName(),
							RemoteControllerUID:       string(objectMeta.GetUID()),
						})
						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("mockSync: -want, +got: %s", diff)
						}

						return nil
					},
				},
			},
			ar:        kubeAR(withTemplate(template(service))),
			secrets:   []corev1.Secret{*secret},
			wantState: v1alpha1.KubernetesApplicationResourceStateSubmitted,
		},
		{
			name:      "MissingTemplate",
			syncer:    &remoteCluster{},
			ar:        kubeAR(),
			wantState: v1alpha1.KubernetesApplicationResourceStateFailed,
			wantErr:   errors.Wrap(errJSON, errUnmarshalTemplate),
		},
		{
			name: "SecretSyncPreservesType",
			syncer: &remoteCluster{
				unstructured: &mockUnstructuredClient{
					mockSync: func(_ context.Context, _ *unstructured.Unstructured) (*v1alpha1.RemoteStatus, error) {
						return remoteStatus, nil
					},
				},
				secret: &mockSecretClient{
					mockSync: func(_ context.Context, got *corev1.Secret) error {
						want := secretWithExplicitType.DeepCopy()
						want.SetName(fmt.Sprintf("%s-%s", objectMeta.GetName(), secretWithExplicitType.GetName()))
						want.SetNamespace(namespace)
						want.SetAnnotations(map[string]string{
							RemoteControllerNamespace: objectMeta.GetNamespace(),
							RemoteControllerName:      objectMeta.GetName(),
							RemoteControllerUID:       string(objectMeta.GetUID()),
						})
						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("mockSync: -want, +got: %s", diff)
						}

						return nil
					},
				},
			},
			ar:        kubeAR(withTemplate(template(service))),
			secrets:   []corev1.Secret{*secretWithExplicitType},
			wantState: v1alpha1.KubernetesApplicationResourceStateSubmitted,
		},
		{
			name: "SecretSyncFailed",
			syncer: &remoteCluster{
				secret: &mockSecretClient{mockSync: newMockSyncSecretFn(errorBoom)},
			},
			ar:        kubeAR(withTemplate(template(serviceWithoutNamespace))),
			secrets:   []corev1.Secret{*secret},
			wantState: v1alpha1.KubernetesApplicationResourceStateFailed,
			wantErr:   errorBoom,
		},
		{
			name: "ResourceSyncFailed",
			syncer: &remoteCluster{
				unstructured: &mockUnstructuredClient{mockSync: newMockSyncUnstructuredFn(nil, errorBoom)},
			},
			ar: kubeAR(
				withTemplate(template(serviceWithoutNamespace)),
				withFinalizers(finalizerName),
				withRemoteStatus(remoteStatus),
			),
			wantState: v1alpha1.KubernetesApplicationResourceStateFailed,
			wantErr:   errorBoom,
		},
		{
			name: "ResourceSyncRefreshedStatusThenFailed",
			syncer: &remoteCluster{
				unstructured: &mockUnstructuredClient{mockSync: newMockSyncUnstructuredFn(remoteStatus, errorBoom)},
			},
			ar:        kubeAR(withTemplate(template(serviceWithoutNamespace))),
			wantState: v1alpha1.KubernetesApplicationResourceStateFailed,
			wantErr:   errorBoom,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotErr := tc.syncer.sync(ctx, tc.ar, tc.secrets)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.syncer.sync(...): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantState, gotState); diff != "" {
				t.Errorf("app: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		name      string
		deleter   deleter
		ar        *v1alpha1.KubernetesApplicationResource
		secrets   []corev1.Secret
		wantState v1alpha1.KubernetesApplicationResourceState
		wantErr   error
	}{
		{
			name: "Successful",
			deleter: &remoteCluster{
				unstructured: &mockUnstructuredClient{
					mockDelete: func(_ context.Context, got *unstructured.Unstructured) error {
						want := template(service)
						want.SetAnnotations(map[string]string{
							RemoteControllerNamespace: objectMeta.GetNamespace(),
							RemoteControllerName:      objectMeta.GetName(),
							RemoteControllerUID:       string(objectMeta.GetUID()),
						})
						if diff := cmp.Diff(want, got); diff != "" {
							errors.Errorf("unstructured mockDelete: -want, +got: %s", diff)
						}

						return nil
					},
				},
				secret: &mockSecretClient{
					mockDelete: func(_ context.Context, got *corev1.Secret) error {
						want := secret.DeepCopy()
						want.SetName(fmt.Sprintf("%s-%s", objectMeta.GetName(), secret.GetName()))
						want.SetNamespace(service.GetNamespace())
						want.SetAnnotations(map[string]string{
							RemoteControllerNamespace: objectMeta.GetNamespace(),
							RemoteControllerName:      objectMeta.GetName(),
							RemoteControllerUID:       string(objectMeta.GetUID()),
						})
						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("secret mockDelete: -want, +got: %s", diff)
						}

						return nil
					},
				},
			},
			ar: kubeAR(
				withFinalizers(finalizerName),
				withTemplate(template(service)),
			),
			secrets:   []corev1.Secret{*secret},
			wantState: v1alpha1.KubernetesApplicationResourceStateSubmitted,
		},
		{
			name:    "MissingTemplate",
			deleter: &remoteCluster{},
			ar: kubeAR(
				withFinalizers(finalizerName),
			),
			wantState: v1alpha1.KubernetesApplicationResourceStateFailed,
			wantErr:   errors.Wrap(errJSON, errUnmarshalTemplate),
		},
		{
			name: "SecretDeleteFailed",
			deleter: &remoteCluster{
				unstructured: &mockUnstructuredClient{mockDelete: newMockDeleteUnstructuredFn(nil)},
				secret:       &mockSecretClient{mockDelete: newMockDeleteSecretFn(errorBoom)},
			},
			ar: kubeAR(
				withFinalizers(finalizerName),
				withTemplate(template(serviceWithoutNamespace)),
			),
			secrets:   []corev1.Secret{*secret},
			wantState: v1alpha1.KubernetesApplicationResourceStateFailed,
			wantErr:   errorBoom,
		},
		{
			name: "ResourceDeleteFailed",
			deleter: &remoteCluster{
				unstructured: &mockUnstructuredClient{mockDelete: newMockDeleteUnstructuredFn(errorBoom)},
			},
			ar: kubeAR(
				withFinalizers(finalizerName),
				withTemplate(template(serviceWithoutNamespace)),
			),
			wantState: v1alpha1.KubernetesApplicationResourceStateFailed,
			wantErr:   errorBoom,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotErr := tc.deleter.delete(ctx, tc.ar, tc.secrets)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.deleter.delete(...): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantState, gotState); diff != "" {
				t.Errorf("AR: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSyncUnstructured(t *testing.T) {
	cases := []struct {
		name         string
		unstructured unstructuredSyncer
		template     *unstructured.Unstructured
		wantStatus   *v1alpha1.RemoteStatus
		wantErr      error
	}{
		{
			name: "Successful",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						// The existing service is slightly different from the
						// updated service because CreateOrUpdate does not call
						// Update if the object did not change.
						existing := template(existingService)
						existing.SetResourceVersion(resourceVersion)
						*obj.(*unstructured.Unstructured) = *existing
						return nil
					},
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						// We compare resource versions to ensure we preserved
						// the existing service's important object metadata.
						want := resourceVersion
						got := obj.(*unstructured.Unstructured).GetResourceVersion()
						if got != want {
							return errors.Errorf("MockUpdate: obj.GetResourceVersion(): want %s, got %s", want, got)
						}
						return nil
					},
				},
			},
			template:   template(service),
			wantStatus: remoteStatus,
			wantErr:    nil,
		},
		{
			name: "ExistingResourceHasDifferentController",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						existing := template(existingService)
						existing.SetAnnotations(map[string]string{})
						*obj.(*unstructured.Unstructured) = *existing
						return nil
					},
				},
			},
			template:   template(service),
			wantStatus: nil,
			wantErr: errors.WithStack(errors.Errorf("cannot sync resource: Service %s/%s exists and is not controlled by %s %s",
				existingService.GetNamespace(),
				existingService.GetName(),
				v1alpha1.KubernetesApplicationResourceKind,
				objectMeta.GetName(),
			)),
		},
		{
			name: "CreateSuccessful",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, name)),
					MockCreate: func(_ context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if diff := cmp.Diff(template(service), obj); diff != "" {
							t.Errorf("Create: -want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			template:   template(service),
			wantStatus: nil,
			wantErr:    nil,
		},
		{
			name: "CreateFailed",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, name)),
					MockCreate: test.NewMockCreateFn(errorBoom),
				},
			},
			template:   template(service),
			wantStatus: nil,
			wantErr:    errors.Wrap(errorBoom, errCreateResource),
		},
		{
			name: "GetFailed",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
			},
			template:   template(service),
			wantStatus: nil,
			wantErr:    errors.Wrap(errorBoom, errGetResource),
		},
		{
			name: "PatchFailed",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(errorBoom),
				},
			},
			template:   template(service),
			wantStatus: remoteStatus,
			wantErr:    errors.Wrap(errors.Wrap(errorBoom, "cannot patch object"), errSyncResource),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotErr := tc.unstructured.sync(ctx, tc.template)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.unstructured.sync(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantStatus, gotStatus); diff != "" {
				t.Errorf("tc.unstructured.sync(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetRemoteStatus(t *testing.T) {
	cases := []struct {
		name   string
		remote runtime.Unstructured
		want   *v1alpha1.RemoteStatus
	}{
		{
			name:   "Successful",
			remote: template(service),
			want:   remoteStatus,
		},
		{
			name:   "MissingStatus",
			remote: &unstructured.Unstructured{},
			want:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := getRemoteStatus(tc.remote)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("getRemoteStatus(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDeleteUnstructured(t *testing.T) {
	cases := []struct {
		name         string
		unstructured unstructuredDeleter
		template     *unstructured.Unstructured
		wantErr      error
	}{
		{
			name: "Successful",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*unstructured.Unstructured) = *(template(existingService))
						return nil
					},
					MockDelete: test.NewMockDeleteFn(nil),
				},
			},
			template: template(service),
			wantErr:  nil,
		},
		{
			name: "ExistingResourceNotFound",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
				},
			},
			template: template(service),
		},
		{
			name: "ExistingResourceHasNoRemoteController",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						existing := template(existingService)
						existing.SetAnnotations(map[string]string{})
						*obj.(*unstructured.Unstructured) = *existing
						return nil
					},
				},
			},
			template: template(service),
		},
		{
			name: "GetExistingResourceFailed",
			unstructured: &unstructuredClient{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
			},
			template: template(service),
			wantErr:  errors.Wrapf(errorBoom, errGetResource),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.unstructured.delete(ctx, tc.template)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.unstructured.delete(...): want error != got error:\n%s", diff)
			}
		})
	}
}

func TestSyncSecret(t *testing.T) {
	cases := []struct {
		name     string
		secret   secretSyncer
		template *corev1.Secret
		wantErr  error
	}{
		{
			name: "Successful",
			secret: &secretClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						// The existing service is slightly different from the
						// updated service because CreateOrUpdate does not call
						// Update if the object did not change.
						existing := existingSecret.DeepCopy()
						existing.SetResourceVersion(resourceVersion)
						*obj.(*corev1.Secret) = *existing
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						// We compare resource versions to ensure we preserved
						// the existing service's important object metadata.
						want := resourceVersion
						got := obj.(*corev1.Secret).GetResourceVersion()
						if got != want {
							return errors.Errorf("MockUpdate: obj.GetResourceVersion(): want %s, got %s", want, got)
						}
						return nil
					},
				},
			},
			template: secret,
			wantErr:  nil,
		},
		{
			name: "ExistingResourceHasDifferentController",
			secret: &secretClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						existing := existingSecret.DeepCopy()
						existing.SetAnnotations(map[string]string{})
						*obj.(*corev1.Secret) = *existing
						return nil
					},
				},
			},
			template: secret,
			wantErr: errors.WithStack(errors.Errorf("cannot sync secret: secret %s/%s exists and is not controlled by %s %s",
				existingSecret.GetNamespace(),
				existingSecret.GetName(),
				v1alpha1.KubernetesApplicationResourceKind,
				objectMeta.GetName(),
			)),
		},
		{
			name: "CreateOrUpdateFailed",
			secret: &secretClient{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errorBoom),
				},
			},
			template: secret,
			wantErr:  errors.Wrap(errorBoom, errSyncSecret),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.secret.sync(ctx, tc.template)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.unstructured.sync(...): want error != got error:\n%s", diff)
			}
		})
	}
}

func TestDeleteSecret(t *testing.T) {
	cases := []struct {
		name     string
		secret   secretDeleter
		template *corev1.Secret
		wantErr  error
	}{
		{
			name: "Successful",
			secret: &secretClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*corev1.Secret) = *existingSecret
						return nil
					},
					MockDelete: test.NewMockDeleteFn(nil),
				},
			},
			template: secret,
			wantErr:  nil,
		},
		{
			name: "ExistingResourceNotFound",
			secret: &secretClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
				},
			},
			template: secret,
		},
		{
			name: "ExistingResourceHasNoRemoteController",
			secret: &secretClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						existing := existingSecret
						existing.SetAnnotations(map[string]string{})
						*obj.(*corev1.Secret) = *existing
						return nil
					},
				},
			},
			template: secret,
		},
		{
			name: "GetExistingResourceFailed",
			secret: &secretClient{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
			},
			template: secret,
			wantErr:  errors.Wrapf(errorBoom, errGetSecret),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.secret.delete(ctx, tc.template)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.secret.delete(...): want error != got error:\n%s", diff)
			}
		})
	}
}

// We pass in a mock RESTMapper when testing in order to prevent
// client.New() trying to create one itself, because creating a new
// RESTMapper involves connecting to the API server.
type mockRESTMapper struct {
	kmeta.RESTMapper
}

func TestConnectConfig(t *testing.T) {
	cases := []struct {
		name       string
		connecter  *clusterConnecter
		ar         *v1alpha1.KubernetesApplicationResource
		wantConfig *rest.Config
		wantErr    error
	}{
		{
			name: "Successful",
			connecter: &clusterConnecter{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, got client.ObjectKey, obj runtime.Object) error {
						switch actual := obj.(type) {
						case *v1alpha1.KubernetesTarget:
							want := client.ObjectKey{
								Namespace: target.GetNamespace(),
								Name:      target.GetName(),
							}
							if diff := cmp.Diff(want, got); diff != "" {
								return errors.Errorf("MockGet(Secret): -want, +got: %s", diff)
							}
							*actual = *target

						case *corev1.Secret:
							want := client.ObjectKey{
								Namespace: target.GetNamespace(),
								Name:      secret.GetName(),
							}
							if diff := cmp.Diff(want, got); diff != "" {
								return errors.Errorf("MockGet(Secret): -want, +got: %s", diff)
							}

							*actual = *secret

						}
						return nil
					},
				},
				options: client.Options{Mapper: mockRESTMapper{}},
			},
			ar: kubeAR(withTarget(target.GetName())),
			wantConfig: &rest.Config{
				Host:     apiServerURL.String(),
				Username: string(secret.Data[runtimev1alpha1.ResourceCredentialsSecretUserKey]),
				Password: string(secret.Data[runtimev1alpha1.ResourceCredentialsSecretPasswordKey]),
				TLSClientConfig: rest.TLSClientConfig{
					ServerName: apiServerURL.Hostname(),
					CAData:     secret.Data[runtimev1alpha1.ResourceCredentialsSecretCAKey],
					CertData:   secret.Data[runtimev1alpha1.ResourceCredentialsSecretClientCertKey],
					KeyData:    secret.Data[runtimev1alpha1.ResourceCredentialsSecretClientKeyKey],
				},
				BearerToken: string(secret.Data[runtimev1alpha1.ResourceCredentialsSecretTokenKey]),
			},
			wantErr: nil,
		},
		{
			name:      "KubernetesApplicationResourceNotScheduled",
			connecter: &clusterConnecter{},
			ar:        kubeAR(),
			wantErr: errors.Errorf("%s %s/%s is not scheduled",
				v1alpha1.KubernetesApplicationResourceKind, objectMeta.GetNamespace(), objectMeta.GetName()),
		},
		{
			name: "GetKubernetesTargetFailed",
			connecter: &clusterConnecter{
				kube:    &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
				options: client.Options{Mapper: mockRESTMapper{}},
			},
			ar: kubeAR(withTarget(target.GetName())),
			wantErr: errors.Wrapf(errorBoom, "cannot get %s %s/%s",
				v1alpha1.KubernetesTargetKind, target.GetNamespace(), target.GetName()),
		},
		{
			name: "GetConnectionSecretFailed",
			connecter: &clusterConnecter{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch actual := obj.(type) {
						case *v1alpha1.KubernetesTarget:
							*actual = *target
						case *corev1.Secret:
							return errorBoom
						}
						return nil
					},
				},
				options: client.Options{Mapper: mockRESTMapper{}},
			},
			ar:      kubeAR(withTarget(target.GetName())),
			wantErr: errors.Wrapf(errorBoom, "cannot get secret %s/%s", secret.GetNamespace(), secret.GetName()),
		},
		{
			name: "ParseEndpointFailed",
			connecter: &clusterConnecter{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch actual := obj.(type) {
						case *v1alpha1.KubernetesTarget:
							*actual = *target
						case *corev1.Secret:
							s := secret.DeepCopy()
							s.Data[runtimev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(malformedURL)
							*actual = *s
						}
						return nil
					},
				},
				options: client.Options{Mapper: mockRESTMapper{}},
			},
			ar:      kubeAR(withTarget(target.GetName())),
			wantErr: errors.WithStack(errors.Wrap(urlParseError(malformedURL), "cannot parse Kubernetes endpoint as URL")),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotConfig, gotErr := tc.connecter.config(ctx, tc.ar)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.connecter.config(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantConfig, gotConfig); diff != "" {
				t.Errorf("tc.connecter.config(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnect(t *testing.T) {
	cases := []struct {
		name      string
		connecter connecter
		ar        *v1alpha1.KubernetesApplicationResource
		wantSD    syncdeleter
		wantErr   error
	}{
		{
			name: "Successful",
			connecter: &clusterConnecter{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						if actual, ok := obj.(*v1alpha1.KubernetesTarget); ok {
							*actual = *target
						}
						return nil
					},
				},
				options: client.Options{Mapper: mockRESTMapper{}},
			},
			ar: kubeAR(withTarget(target.GetName())),

			// This empty struct is 'identical' to the actual, populated struct
			// returned by tc.connecter.connect() because we do not compare
			// unexported fields. We don't inspect these unexported fields
			// because doing so would mostly be testing controller-runtime's
			// client.New() code, not ours.
			wantSD:  &remoteCluster{},
			wantErr: nil,
		},
		{
			name: "MissingConnectionSecret",
			connecter: &clusterConnecter{
				kube:    &test.MockClient{MockGet: test.NewMockGetFn(nil)},
				options: client.Options{Mapper: mockRESTMapper{}},
			},
			ar: kubeAR(withTarget(target.GetName())),

			// This empty struct is 'identical' to the actual, populated struct
			// returned by tc.connecter.connect() because we do not compare
			// unexported fields. We don't inspect these unexported fields
			// because doing so would mostly be testing controller-runtime's
			// client.New() code, not ours.
			wantErr: errors.Wrap(
				errors.Errorf("%s %s/%s has no connection secret", v1alpha1.KubernetesTargetKind, target.GetNamespace(), target.GetName()),
				"cannot create Kubernetes client configuration"),
		},
		{
			name: "ConfigFailure",
			connecter: &clusterConnecter{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
			},
			ar: kubeAR(withTarget(target.GetName())),
			wantErr: errors.Wrapf(errorBoom, "cannot create Kubernetes client configuration: cannot get %s %s/%s",
				v1alpha1.KubernetesTargetKind, target.GetNamespace(), target.GetName()),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			gotSD, gotErr := tc.connecter.connect(ctx, tc.ar)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.connecter.connect(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantSD, gotSD, cmpopts.IgnoreUnexported(remoteCluster{})); diff != "" {
				t.Errorf("tc.connecter.connect(...): -want, +got:\n%s", diff)
			}
		})
	}
}

type mockSyncFn func(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error)

func newMockSyncFn(s v1alpha1.KubernetesApplicationResourceState, e error) mockSyncFn {
	return func(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error) {
		return s, e
	}
}

type mockDeleteFn func(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error)

func newMockDeleteFn(s v1alpha1.KubernetesApplicationResourceState, e error) mockDeleteFn {
	return func(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error) {
		return s, e
	}
}

type mockSyncDeleter struct {
	mockSync   mockSyncFn
	mockDelete mockDeleteFn
}

func (m *mockSyncDeleter) sync(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error) {
	return m.mockSync(ctx, ar, secrets)
}

func (m *mockSyncDeleter) delete(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error) {
	return m.mockDelete(ctx, ar, secrets)
}

var noopSyncDeleter = &mockSyncDeleter{
	mockSync:   newMockSyncFn(v1alpha1.KubernetesApplicationResourceStateSubmitted, nil),
	mockDelete: newMockDeleteFn(v1alpha1.KubernetesApplicationResourceStateSubmitted, nil),
}

type mockConnectFn func(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource) (syncdeleter, error)

func newMockConnectFn(sd syncdeleter, err error) mockConnectFn {
	return func(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource) (syncdeleter, error) {
		return sd, err
	}
}

type mockConnecter struct {
	mockConnect mockConnectFn
}

func (m *mockConnecter) connect(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource) (syncdeleter, error) {
	return m.mockConnect(ctx, ar)
}

func TestReconcile(t *testing.T) {
	cases := []struct {
		name       string
		rec        *Reconciler
		req        reconcile.Request
		wantResult reconcile.Result
		wantErr    error
	}{
		{
			name: "FailedToGetNonExistentKAR",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, _ runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)

						want := kubeAR(
							withConditions(runtimev1alpha1.ReconcileError(errorBoom)),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockStatusUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		{
			name: "FailedToGetExtantKAR",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errorBoom),
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)

						want := kubeAR(
							withConditions(runtimev1alpha1.ReconcileError(errorBoom)),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockStatusUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: shortWait},
			wantErr:    errors.Wrapf(errorBoom, "cannot get %s %s/%s", v1alpha1.KubernetesApplicationResourceKind, namespace, name),
		},
		{
			name: "FailedToConnect",
			rec: &Reconciler{
				connecter: &mockConnecter{mockConnect: newMockConnectFn(nil, errorBoom)},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplicationResource) = *(kubeAR())
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)
						want := kubeAR(withFinalizers(finalizerName))

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockUpdate: -want, +got: %s", diff)
						}

						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)

						want := kubeAR(
							withFinalizers(finalizerName),
							withConditions(runtimev1alpha1.ReconcileError(errorBoom)),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockStatusUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: shortWait},
		},
		{
			name: "KARDeletedButCannotConnect",
			rec: &Reconciler{
				connecter: &mockConnecter{mockConnect: newMockConnectFn(nil, errors.Wrap(kerrors.NewNotFound(schema.GroupResource{}, ""), ""))},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplicationResource) = *(kubeAR(
							withFinalizers(finalizerName),
							withDeletionTimestamp(deleteTime)))
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)
						want := kubeAR(
							withDeletionTimestamp(deleteTime),
						)

						// We ignore finalizers because the first update call
						// should have one but the second should not.
						if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(v1alpha1.KubernetesApplicationResource{}, "ObjectMeta.Finalizers")); diff != "" {
							return errors.Errorf("MockUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
		},
		{
			name: "KARDeletedSuccessfully",
			rec: &Reconciler{
				connecter: &mockConnecter{mockConnect: newMockConnectFn(noopSyncDeleter, nil)},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplicationResource) = *(kubeAR(withDeletionTimestamp(deleteTime)))
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)
						want := kubeAR(
							withDeletionTimestamp(deleteTime),
						)

						// We ignore finalizers because the first update call
						// should have one but the second should not.
						if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(v1alpha1.KubernetesApplicationResource{}, "ObjectMeta.Finalizers")); diff != "" {
							return errors.Errorf("MockUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
		},
		{
			name: "KARDeleteFailure",
			rec: &Reconciler{
				connecter: &mockConnecter{mockConnect: newMockConnectFn(&mockSyncDeleter{
					mockSync:   newMockSyncFn(v1alpha1.KubernetesApplicationResourceStateSubmitted, nil),
					mockDelete: newMockDeleteFn(v1alpha1.KubernetesApplicationResourceStateFailed, errorBoom),
				}, nil)},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplicationResource) = *(kubeAR(withDeletionTimestamp(deleteTime)))
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)
						want := kubeAR(
							withDeletionTimestamp(deleteTime),
							withFinalizers(finalizerName),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockUpdate: -want, +got: %s", diff)
						}

						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)

						want := kubeAR(
							withFinalizers(finalizerName),
							withDeletionTimestamp(deleteTime),
							withState(v1alpha1.KubernetesApplicationResourceStateFailed),
							withConditions(runtimev1alpha1.ReconcileError(errorBoom)),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockStatusUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: shortWait},
		},
		{
			name: "KARSyncedSuccessfully",
			rec: &Reconciler{
				connecter: &mockConnecter{mockConnect: newMockConnectFn(noopSyncDeleter, nil)},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplicationResource) = *(kubeAR())
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)
						want := kubeAR(withFinalizers(finalizerName))

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockUpdate: -want, +got: %s", diff)
						}

						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)

						want := kubeAR(
							withFinalizers(finalizerName),
							withState(v1alpha1.KubernetesApplicationResourceStateSubmitted),
							withConditions(runtimev1alpha1.ReconcileSuccess()),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockStatusUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: longWait},
		},
		{
			name: "KARSyncFailure",
			rec: &Reconciler{
				connecter: &mockConnecter{mockConnect: newMockConnectFn(&mockSyncDeleter{
					mockSync:   newMockSyncFn(v1alpha1.KubernetesApplicationResourceStateFailed, errorBoom),
					mockDelete: newMockDeleteFn(v1alpha1.KubernetesApplicationResourceStateSubmitted, nil),
				}, nil)},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplicationResource) = *(kubeAR())
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)
						want := kubeAR(withFinalizers(finalizerName))

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockUpdate: -want, +got: %s", diff)
						}

						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplicationResource)

						want := kubeAR(
							withFinalizers(finalizerName),
							withState(v1alpha1.KubernetesApplicationResourceStateFailed),
							withConditions(runtimev1alpha1.ReconcileError(errorBoom)),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockStatusUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: shortWait},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, gotErr := tc.rec.Reconcile(tc.req)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantResult, gotResult); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestGetConnectionSecrets(t *testing.T) {
	cases := []struct {
		name        string
		rec         *Reconciler
		ar          *v1alpha1.KubernetesApplicationResource
		wantAR      *v1alpha1.KubernetesApplicationResource
		wantSecrets []corev1.Secret
	}{
		{
			name: "Successful",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*obj.(*corev1.Secret) = *secret
						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			ar: kubeAR(
				withSecrets(secretLocalObjectRef),
			),
			wantAR: kubeAR(
				withSecrets(secretLocalObjectRef),
			),
			wantSecrets: []corev1.Secret{*secret},
		},
		{
			name: "Failed",
			rec: &Reconciler{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
				log:  logging.NewNopLogger(),
			},
			ar: kubeAR(
				withSecrets(secretLocalObjectRef),
			),
			wantAR: kubeAR(
				withSecrets(secretLocalObjectRef),
				withConditions(runtimev1alpha1.ReconcileError(errorBoom)),
			),
			wantSecrets: []corev1.Secret{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSecrets := tc.rec.getConnectionSecrets(ctx, tc.ar)

			if diff := cmp.Diff(tc.wantSecrets, gotSecrets); diff != "" {
				t.Errorf("tc.rec.getConnectionSecrets(...): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantAR, tc.ar); diff != "" {
				t.Errorf("AR: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestHasController(t *testing.T) {
	cases := []struct {
		name string
		obj  metav1.Object
		want bool
	}{
		{
			name: "HasController",
			obj:  service,
			want: true,
		},
		{
			name: "MissingNamespace",
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						RemoteControllerName: name,
						RemoteControllerUID:  string(uid),
					},
				},
			},
			want: false,
		},
		{
			name: "MissingName",
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						RemoteControllerNamespace: namespace,
						RemoteControllerUID:       string(uid),
					},
				},
			},
			want: false,
		},
		{
			name: "MissingUID",
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						RemoteControllerNamespace: namespace,
						RemoteControllerName:      name,
					},
				},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hasController(tc.obj)
			if got != tc.want {
				t.Errorf("hasController(...): want %t, got %t", tc.want, got)
			}
		})
	}
}

func TestHaveSameController(t *testing.T) {
	cases := []struct {
		name string
		a    metav1.Object
		b    metav1.Object
		want bool
	}{
		{
			name: "HasSameController",
			a:    service,
			b:    existingService,
			want: true,
		},
		{
			name: "HasNoController",
			a:    &corev1.Service{},
			b:    existingService,
			want: false,
		},
		{
			name: "HasDifferentUID",
			a:    service,
			b: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						RemoteControllerNamespace: namespace,
						RemoteControllerName:      name,
						RemoteControllerUID:       "imdifferent!",
					},
				},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := haveSameController(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("hasController(...): want %t, got %t", tc.want, got)
			}
		})
	}
}
