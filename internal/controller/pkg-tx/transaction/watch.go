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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
)

// EnqueueIncompleteTransactionsForLock enqueues all incomplete Transactions
// when the Lock changes. This allows Transactions waiting for lock acquisition
// to be reconciled immediately when the lock becomes available.
func EnqueueIncompleteTransactionsForLock(kube client.Client, log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		transactions := &v1alpha1.TransactionList{}
		if err := kube.List(ctx, transactions); err != nil {
			log.Debug("Cannot list transactions while attempting to enqueue requests", "error", err)
			return nil
		}

		var requests []reconcile.Request
		for _, tx := range transactions.Items {
			if !tx.IsComplete() {
				log.Debug("Enqueuing incomplete transaction for lock change", "transaction", tx.GetName())
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: tx.GetName()},
				})
			}
		}

		return requests
	})
}
