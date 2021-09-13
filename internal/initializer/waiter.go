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

package initializer

import (
	"context"
	"time"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const (
	errGetCRD             = "cannot get crd"
	errFmtTimeoutExceeded = "%f seconds timeout for waiting CRDs to be ready is exceeded"
)

// NewCRDWaiter returns a new *CRDWaiter initializer.
func NewCRDWaiter(names []string, timeout time.Duration, period time.Duration, log logging.Logger) *CRDWaiter {
	return &CRDWaiter{Names: names, Timeout: timeout, Period: period, log: log}
}

// CRDWaiter blocks the execution until all the CRDs whose names are given are
// deployed to the cluster.
type CRDWaiter struct {
	Names   []string
	Timeout time.Duration
	Period  time.Duration
	log     logging.Logger
}

// Run continuously checks whether the list of CRDs whose names are given are
// present in the cluster.
func (cw *CRDWaiter) Run(ctx context.Context, kube client.Client) error {
	timeout := time.After(cw.Timeout)
	ticker := time.NewTicker(cw.Period)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cw.log.Info("Waiting for required CRDs to be present", "names", cw.Names, "poll-interval", cw.Period)
			present := 0
			for _, n := range cw.Names {
				crd := &v1.CustomResourceDefinition{}
				nn := types.NamespacedName{Name: n}
				err := kube.Get(ctx, nn, crd)
				if err != nil && !kerrors.IsNotFound(err) {
					return errors.Wrap(err, errGetCRD)
				}
				if kerrors.IsNotFound(err) {
					break
				}
				present++
			}
			if present == len(cw.Names) {
				return nil
			}
		case <-timeout:
			return errors.Errorf(errFmtTimeoutExceeded, cw.Timeout.Seconds())
		}
	}
}
