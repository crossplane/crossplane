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

// Package transaction implements the Transaction controller.
package transaction

import (
	"context"
	"strconv"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

const (
	// We have a singleton Lock - it's always named 'lock'.
	lockName = "lock"

	// lockTimeout is how long a lock can be held before it's considered
	// orphaned and can be taken over by another transaction. This should be
	// longer than the reconcile timeout to allow transactions to complete.
	lockTimeout = 10 * time.Minute
)

var (
	// ErrLockHeldByAnotherTransaction is returned when attempting to acquire a lock held by another Transaction.
	ErrLockHeldByAnotherTransaction = errors.New("lock is held by another transaction")
	// ErrTransactionDoesNotHoldLock is returned when attempting to commit/release a lock not held by the Transaction.
	ErrTransactionDoesNotHoldLock = errors.New("transaction does not hold the lock")
)

// AtomicLockManager implements LockManager using the Kubernetes API.
type AtomicLockManager struct {
	client client.Client
}

// NewAtomicLockManager creates a new APILockManager.
func NewAtomicLockManager(c client.Client) *AtomicLockManager {
	return &AtomicLockManager{client: c}
}

// Acquire attempts to gain exclusive access to the Lock for a Transaction.
func (m *AtomicLockManager) Acquire(ctx context.Context, tx *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
	lock := &v1beta1.Lock{}
	if err := m.client.Get(ctx, types.NamespacedName{Name: lockName}, lock); err != nil {
		return nil, errors.Wrap(err, "cannot get lock")
	}

	current := lock.GetAnnotations()[v1beta1.AnnotationCurrentTransaction]

	if current == tx.GetName() {
		return lock.Packages, nil
	}

	if current != "" {
		// Check if lock has expired based on timestamp
		if acquiredAt := lock.GetAnnotations()[v1beta1.AnnotationLockAcquiredAt]; acquiredAt != "" {
			t, err := time.Parse(time.RFC3339, acquiredAt)
			if err != nil {
				return nil, errors.Wrap(err, "cannot parse lock acquired timestamp")
			}
			if time.Since(t) < lockTimeout {
				// Lock is still valid
				return nil, ErrLockHeldByAnotherTransaction
			}
			// Lock has expired, allow takeover
		} else {
			// No timestamp means old lock format or corrupted - be conservative
			// and don't allow takeover
			return nil, ErrLockHeldByAnotherTransaction
		}
	}

	if lock.Annotations == nil {
		lock.Annotations = make(map[string]string)
	}
	lock.Annotations[v1beta1.AnnotationCurrentTransaction] = tx.Name
	lock.Annotations[v1beta1.AnnotationLockAcquiredAt] = time.Now().Format(time.RFC3339)

	nextTxNum := int64(1)
	if numStr := lock.Annotations[v1beta1.AnnotationNextTransactionNumber]; numStr != "" {
		var err error
		nextTxNum, err = strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "cannot parse next transaction number")
		}
	}

	tx.Status.TransactionNumber = nextTxNum
	lock.Annotations[v1beta1.AnnotationNextTransactionNumber] = strconv.FormatInt(nextTxNum+1, 10)

	if err := m.client.Update(ctx, lock); err != nil {
		if kerrors.IsConflict(err) {
			return nil, ErrLockHeldByAnotherTransaction
		}
		return nil, errors.Wrap(err, "cannot acquire lock")
	}

	return lock.Packages, nil
}

// Commit releases exclusive access and updates Lock state with new packages.
func (m *AtomicLockManager) Commit(ctx context.Context, tx *v1alpha1.Transaction, packages []v1beta1.LockPackage) error {
	lock := &v1beta1.Lock{}
	if err := m.client.Get(ctx, types.NamespacedName{Name: lockName}, lock); err != nil {
		return errors.Wrap(err, "cannot get lock")
	}

	current := lock.GetAnnotations()[v1beta1.AnnotationCurrentTransaction]
	if current != tx.GetName() {
		return ErrTransactionDoesNotHoldLock
	}

	lock.Packages = packages
	delete(lock.Annotations, v1beta1.AnnotationCurrentTransaction)
	delete(lock.Annotations, v1beta1.AnnotationLockAcquiredAt)

	if err := m.client.Update(ctx, lock); err != nil {
		return errors.Wrap(err, "cannot release lock")
	}

	return nil
}

// Release releases the lock without updating packages.
// Returns nil if successfully released or if the Transaction never held the lock.
func (m *AtomicLockManager) Release(ctx context.Context, tx *v1alpha1.Transaction) error {
	lock := &v1beta1.Lock{}
	if err := m.client.Get(ctx, types.NamespacedName{Name: lockName}, lock); err != nil {
		return errors.Wrap(err, "cannot get lock")
	}

	current := lock.GetAnnotations()[v1beta1.AnnotationCurrentTransaction]
	if current != tx.GetName() {
		// Transaction doesn't hold the lock - nothing to release
		return nil
	}

	delete(lock.Annotations, v1beta1.AnnotationCurrentTransaction)
	delete(lock.Annotations, v1beta1.AnnotationLockAcquiredAt)

	if err := m.client.Update(ctx, lock); err != nil {
		return errors.Wrap(err, "cannot release lock")
	}

	return nil
}
