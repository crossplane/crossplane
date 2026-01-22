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

package managed

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	kmetrics "k8s.io/component-base/metrics"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

const subSystem = "crossplane"

// MetricRecorder records the managed resource metrics.
type MetricRecorder interface { //nolint:interfacebloat // The first two methods are coming from Prometheus
	Describe(ch chan<- *prometheus.Desc)
	Collect(ch chan<- prometheus.Metric)

	recordUnchanged(name string)
	recordFirstTimeReconciled(managed resource.Managed)
	recordFirstTimeReady(managed resource.Managed)
	recordDrift(managed resource.Managed)
	recordDeleted(managed resource.Managed)
}

// MRMetricRecorder records the lifecycle metrics of managed resources.
type MRMetricRecorder struct {
	firstObservation sync.Map
	lastObservation  sync.Map

	mrDetected       *prometheus.HistogramVec
	mrFirstTimeReady *prometheus.HistogramVec
	mrDeletion       *prometheus.HistogramVec
	mrDrift          *prometheus.HistogramVec
}

// NewMRMetricRecorder returns a new MRMetricRecorder which records metrics for managed resources.
func NewMRMetricRecorder() *MRMetricRecorder {
	return &MRMetricRecorder{
		mrDetected: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: subSystem,
			Name:      "managed_resource_first_time_to_reconcile_seconds",
			Help:      "The time it took for a managed resource to be detected by the controller",
			Buckets:   kmetrics.ExponentialBuckets(10e-9, 10, 10),
		}, []string{"gvk"}),
		mrFirstTimeReady: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: subSystem,
			Name:      "managed_resource_first_time_to_readiness_seconds",
			Help:      "The time it took for a managed resource to become ready first time after creation",
			Buckets:   []float64{1, 5, 10, 15, 30, 60, 120, 300, 600, 1800, 3600},
		}, []string{"gvk"}),
		mrDeletion: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: subSystem,
			Name:      "managed_resource_deletion_seconds",
			Help:      "The time it took for a managed resource to be deleted",
			Buckets:   []float64{1, 5, 10, 15, 30, 60, 120, 300, 600, 1800, 3600},
		}, []string{"gvk"}),
		mrDrift: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: subSystem,
			Name:      "managed_resource_drift_seconds",
			Help:      "ALPHA: How long since the previous successful reconcile when a resource was found to be out of sync; excludes restart of the provider",
			Buckets:   kmetrics.ExponentialBuckets(10e-9, 10, 10),
		}, []string{"gvk"}),
	}
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (r *MRMetricRecorder) Describe(ch chan<- *prometheus.Desc) {
	r.mrDetected.Describe(ch)
	r.mrFirstTimeReady.Describe(ch)
	r.mrDeletion.Describe(ch)
	r.mrDrift.Describe(ch)
}

// Collect is called by the Prometheus registry when collecting
// metrics. The implementation sends each collected metric via the
// provided channel and returns once the last metric has been sent.
func (r *MRMetricRecorder) Collect(ch chan<- prometheus.Metric) {
	r.mrDetected.Collect(ch)
	r.mrFirstTimeReady.Collect(ch)
	r.mrDeletion.Collect(ch)
	r.mrDrift.Collect(ch)
}

func (r *MRMetricRecorder) recordUnchanged(name string) {
	r.lastObservation.Store(name, time.Now())
}

func (r *MRMetricRecorder) recordFirstTimeReconciled(managed resource.Managed) {
	if managed.GetCondition(xpv1.TypeSynced).Status == corev1.ConditionUnknown {
		r.mrDetected.With(getLabels(managed)).Observe(time.Since(managed.GetCreationTimestamp().Time).Seconds())
		r.firstObservation.Store(managed.GetName(), time.Now()) // this is the first time we reconciled on this resource
	}
}

func (r *MRMetricRecorder) recordDrift(managed resource.Managed) {
	name := managed.GetName()

	last, ok := r.lastObservation.Load(name)
	if !ok {
		return
	}

	lt, ok := last.(time.Time)
	if !ok {
		return
	}

	r.mrDrift.With(getLabels(managed)).Observe(time.Since(lt).Seconds())

	r.lastObservation.Store(name, time.Now())
}

func (r *MRMetricRecorder) recordDeleted(managed resource.Managed) {
	r.mrDeletion.With(getLabels(managed)).Observe(time.Since(managed.GetDeletionTimestamp().Time).Seconds())
}

func (r *MRMetricRecorder) recordFirstTimeReady(managed resource.Managed) {
	// Note that providers may set the ready condition to "True", so we need
	// to check the value here to send the ready metric
	if managed.GetCondition(xpv1.TypeReady).Status == corev1.ConditionTrue {
		_, ok := r.firstObservation.Load(managed.GetName()) // This map is used to identify the first time to readiness
		if !ok {
			return
		}

		r.mrFirstTimeReady.With(getLabels(managed)).Observe(time.Since(managed.GetCreationTimestamp().Time).Seconds())
		r.firstObservation.Delete(managed.GetName())
	}
}

// A NopMetricRecorder does nothing.
type NopMetricRecorder struct{}

// NewNopMetricRecorder returns a MRMetricRecorder that does nothing.
func NewNopMetricRecorder() *NopMetricRecorder {
	return &NopMetricRecorder{}
}

// Describe does nothing.
func (r *NopMetricRecorder) Describe(_ chan<- *prometheus.Desc) {}

// Collect does nothing.
func (r *NopMetricRecorder) Collect(_ chan<- prometheus.Metric) {}

func (r *NopMetricRecorder) recordUnchanged(_ string) {}

func (r *NopMetricRecorder) recordFirstTimeReconciled(_ resource.Managed) {}

func (r *NopMetricRecorder) recordDrift(_ resource.Managed) {}

func (r *NopMetricRecorder) recordDeleted(_ resource.Managed) {}

func (r *NopMetricRecorder) recordFirstTimeReady(_ resource.Managed) {}

func getLabels(r resource.Managed) prometheus.Labels {
	return prometheus.Labels{
		"gvk": r.GetObjectKind().GroupVersionKind().String(),
	}
}
