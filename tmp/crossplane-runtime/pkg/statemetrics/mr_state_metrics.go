/*
Copyright 2024 The Crossplane Authors.

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

package statemetrics

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// MRStateMetrics holds Prometheus metrics for managed resources.
type MRStateMetrics struct {
	Exists *prometheus.GaugeVec
	Ready  *prometheus.GaugeVec
	Synced *prometheus.GaugeVec
}

// NewMRStateMetrics returns a new MRStateMetrics.
func NewMRStateMetrics() *MRStateMetrics {
	return &MRStateMetrics{
		Exists: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "managed_resource_exists",
			Help:      "The number of managed resources that exist",
		}, []string{"gvk"}),
		Ready: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "managed_resource_ready",
			Help:      "The number of managed resources in Ready=True state",
		}, []string{"gvk"}),
		Synced: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "managed_resource_synced",
			Help:      "The number of managed resources in Synced=True state",
		}, []string{"gvk"}),
	}
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (r *MRStateMetrics) Describe(ch chan<- *prometheus.Desc) {
	r.Exists.Describe(ch)
	r.Ready.Describe(ch)
	r.Synced.Describe(ch)
}

// Collect is called by the Prometheus registry when collecting
// metrics. The implementation sends each collected metric via the
// provided channel and returns once the last metric has been sent.
func (r *MRStateMetrics) Collect(ch chan<- prometheus.Metric) {
	r.Exists.Collect(ch)
	r.Ready.Collect(ch)
	r.Synced.Collect(ch)
}

// A MRStateRecorder records the state of managed resources.
type MRStateRecorder struct {
	client      client.Client
	log         logging.Logger
	interval    time.Duration
	managedList resource.ManagedList

	metrics *MRStateMetrics
}

// NewMRStateRecorder returns a new MRStateRecorder which records the state of managed resources.
func NewMRStateRecorder(c client.Client, log logging.Logger, metrics *MRStateMetrics, managedList resource.ManagedList, interval time.Duration) *MRStateRecorder {
	return &MRStateRecorder{
		client:      c,
		log:         log,
		metrics:     metrics,
		managedList: managedList,
		interval:    interval,
	}
}

// Record records the state of managed resources.
func (r *MRStateRecorder) Record(ctx context.Context) error {
	if err := r.client.List(ctx, r.managedList); err != nil {
		return errors.Wrap(err, "failed to list managed resources")
	}

	labels, err := r.getLabels()
	if err != nil {
		return errors.Wrap(err, "failed to get labels")
	}

	mrs := r.managedList.GetItems()
	r.metrics.Exists.With(labels).Set(float64(len(mrs)))

	var numReady, numSynced float64 = 0, 0

	for _, o := range mrs {
		if o.GetCondition(xpv1.TypeReady).Status == corev1.ConditionTrue {
			numReady++
		}

		if o.GetCondition(xpv1.TypeSynced).Status == corev1.ConditionTrue {
			numSynced++
		}
	}

	r.metrics.Ready.With(labels).Set(numReady)
	r.metrics.Synced.With(labels).Set(numSynced)

	return nil
}

// Start records state of managed resources with given interval.
func (r *MRStateRecorder) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)

	for {
		select {
		case <-ticker.C:
			if err := r.Record(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			ticker.Stop()
			return nil
		}
	}
}

func (r *MRStateRecorder) getLabels() (prometheus.Labels, error) {
	gvk, err := apiutil.GVKForObject(r.managedList, r.client.Scheme())
	if err != nil {
		return nil, err
	}

	// Remove "List" to get object kind.
	res := strings.Replace(gvk.String(), "List", "", 1)

	return prometheus.Labels{"gvk": res}, nil
}
