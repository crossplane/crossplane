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

package transaction

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

func TestAcquire(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		kube client.Client
		tx   *v1alpha1.Transaction
	}
	type want struct {
		packages []v1beta1.LockPackage
		txNumber int64
		err      error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FirstAcquire": {
			reason: "Should acquire lock and set transaction number to 1",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.Packages = []v1beta1.LockPackage{
							{Source: "xpkg.io/crossplane/provider-aws", Version: "v1.0.0"},
						}
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				packages: []v1beta1.LockPackage{
					{Source: "xpkg.io/crossplane/provider-aws", Version: "v1.0.0"},
				},
				txNumber: 1,
				err:      nil,
			},
		},
		"ReacquireSameLock": {
			reason: "Should return current packages when transaction already holds lock",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-1",
						})
						lock.Packages = []v1beta1.LockPackage{
							{Source: "xpkg.io/crossplane/provider-aws", Version: "v1.0.0"},
						}
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				packages: []v1beta1.LockPackage{
					{Source: "xpkg.io/crossplane/provider-aws", Version: "v1.0.0"},
				},
				txNumber: 0,
				err:      nil,
			},
		},
		"LockHeldWithValidTimestamp": {
			reason: "Should return error when lock is held with recent timestamp",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-other",
							v1beta1.AnnotationLockAcquiredAt:     time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
						})
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: ErrLockHeldByAnotherTransaction,
			},
		},
		"ExpiredLockRecovery": {
			reason: "Should acquire lock when timestamp has expired",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-expired",
							v1beta1.AnnotationLockAcquiredAt:     time.Now().Add(-15 * time.Minute).Format(time.RFC3339),
						})
						lock.Packages = []v1beta1.LockPackage{
							{Source: "xpkg.io/crossplane/provider-aws", Version: "v1.0.0"},
						}
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				packages: []v1beta1.LockPackage{
					{Source: "xpkg.io/crossplane/provider-aws", Version: "v1.0.0"},
				},
				txNumber: 1,
				err:      nil,
			},
		},
		"ConflictOnAcquire": {
			reason: "Should return error when lock is acquired by another transaction concurrently",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(kerrors.NewConflict(schema.GroupResource{}, lockName, errors.New("conflict"))),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				txNumber: 1,
				err:      ErrLockHeldByAnotherTransaction,
			},
		},
		"InvalidTransactionNumber": {
			reason: "Should return error when next transaction number cannot be parsed",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationNextTransactionNumber: "not-a-number",
						})
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"LockHeldWithMissingTimestamp": {
			reason: "Should return error when lock is held but missing timestamp (conservative)",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-other",
							// Missing AnnotationLockAcquiredAt
						})
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: ErrLockHeldByAnotherTransaction,
			},
		},
		"LockHeldWithInvalidTimestamp": {
			reason: "Should return error when lock timestamp cannot be parsed",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-other",
							v1beta1.AnnotationLockAcquiredAt:     "not-a-timestamp",
						})
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"TransactionNumberIncrement": {
			reason: "Should increment transaction number correctly",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationNextTransactionNumber: "42",
						})
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						if diff := cmp.Diff("43", lock.GetAnnotations()[v1beta1.AnnotationNextTransactionNumber]); diff != "" {
							t.Errorf("next transaction number: -want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				txNumber: 42,
				err:      nil,
			},
		},
		"GetLockError": {
			reason: "Should return error when lock cannot be retrieved",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"UpdateLockError": {
			reason: "Should return error when lock update fails",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				txNumber: 1,
				err:      cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &AtomicLockManager{client: tc.args.kube}
			packages, err := m.Acquire(context.Background(), tc.args.tx)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nAcquire(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.packages, packages); diff != "" {
				t.Errorf("%s\nAcquire(...): -want packages, +got packages:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.txNumber, tc.args.tx.Status.TransactionNumber); diff != "" {
				t.Errorf("%s\nAcquire(...): -want transaction number, +got transaction number:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCommit(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		kube     client.Client
		tx       *v1alpha1.Transaction
		packages []v1beta1.LockPackage
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should commit packages and release lock",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-1",
						})
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						wantPackages := []v1beta1.LockPackage{
							{Source: "xpkg.io/crossplane/provider-aws", Version: "v2.0.0"},
						}
						if diff := cmp.Diff(wantPackages, lock.Packages); diff != "" {
							t.Errorf("lock packages: -want, +got:\n%s", diff)
						}
						if _, exists := lock.GetAnnotations()[v1beta1.AnnotationCurrentTransaction]; exists {
							t.Errorf("current transaction annotation should be deleted")
						}
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
				packages: []v1beta1.LockPackage{
					{Source: "xpkg.io/crossplane/provider-aws", Version: "v2.0.0"},
				},
			},
			want: want{
				err: nil,
			},
		},
		"TransactionDoesNotHoldLock": {
			reason: "Should return error when transaction doesn't hold lock",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-other",
						})
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
				packages: []v1beta1.LockPackage{},
			},
			want: want{
				err: ErrTransactionDoesNotHoldLock,
			},
		},
		"GetLockError": {
			reason: "Should return error when lock cannot be retrieved",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
				packages: []v1beta1.LockPackage{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"UpdateLockError": {
			reason: "Should return error when lock update fails",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-1",
						})
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
				packages: []v1beta1.LockPackage{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &AtomicLockManager{client: tc.args.kube}
			err := m.Commit(context.Background(), tc.args.tx, tc.args.packages)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nCommit(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRelease(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		kube client.Client
		tx   *v1alpha1.Transaction
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should release lock successfully",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-1",
						})
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						if _, exists := lock.GetAnnotations()[v1beta1.AnnotationCurrentTransaction]; exists {
							t.Errorf("current transaction annotation should be deleted")
						}
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"TransactionDoesNotHoldLock": {
			reason: "Should succeed when transaction doesn't hold lock (idempotent)",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-other",
						})
						return nil
					}),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"GetLockError": {
			reason: "Should return error when lock cannot be retrieved",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"UpdateLockError": {
			reason: "Should return error when lock update fails",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						lock := obj.(*v1beta1.Lock)
						lock.SetName(lockName)
						lock.SetAnnotations(map[string]string{
							v1beta1.AnnotationCurrentTransaction: "tx-1",
						})
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-1",
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &AtomicLockManager{client: tc.args.kube}
			err := m.Release(context.Background(), tc.args.tx)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nRelease(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
