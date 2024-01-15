/*
Copyright 2023 The Crossplane Authors.

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

// Package top contains the top command.
package top

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const (
	errKubeConfig             = "failed to get kubeconfig"
	errCreateK8sClientset     = "could not create the clientset for Kubernetes"
	errCreateMetricsClientset = "could not create the clientset for Metrics"
	errFetchAllPods           = "could not fetch pods"
	errGetPodMetrics          = "error getting metrics for pod"
	errPrintingPodsTable      = "error creating pods table"
	errAddingPodMetrics       = "error adding metrics to pod, check if metrics-server is running or wait until metrics are available for the pod"
	errWriteHeader            = "cannot write header"
	errWriteRow               = "cannot write row"
)

// Cmd represents the top command.
type Cmd struct {
	Summary   bool   `short:"s" name:"summary" help:"Adds summary header for all Crossplane pods."`
	Namespace string `short:"n" name:"namespace" help:"Show pods from a specific namespace, defaults to crossplane-system." default:"crossplane-system"`
}

// Help returns help instructions for the top command.
func (c *Cmd) Help() string {
	return `
This command returns current resources utilization (CPU and Memory) by Crossplane pods.

Similar to kubectl top pods, it requires Metrics Server to be correctly configured and working on the server.

Examples:
  # Show resources utilization for all Crossplane pods in the default 'crossplane-system' namespace in a tabular format.
  crossplane beta top

  # Show resources utilization for all Crossplane pods in a specified namespace in a tabular format.
  crossplane beta top -n <namespace>

  # Add summary of resources utilization for all Crossplane pods in the default 'crossplane-system' on top of the results.
  crossplane beta top -s
`
}

type topMetrics struct {
	PodType      string
	PodName      string
	PodNamespace string
	CPUUsage     resource.Quantity
	MemoryUsage  resource.Quantity
}

type defaultPrinterRow struct {
	podType   string
	namespace string
	name      string
	cpu       string
	memory    string
}

func (r *defaultPrinterRow) String() string {
	return strings.Join([]string{
		r.podType,
		r.namespace,
		r.name,
		r.cpu,
		r.memory,
	}, "\t")
}

// Run runs the top command.
func (c *Cmd) Run(k *kong.Context, logger logging.Logger) error { //nolint:gocyclo // TODO:(piotr1215) refactor to use dedicated functions
	logger = logger.WithValues("cmd", "top")

	logger.Debug("Tabwriter header created")

	// Build the config from the kubeconfig path
	config, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")

	// Create the clientset for Kubernetes
	k8sClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, errCreateK8sClientset)
	}
	logger.Debug("Created clientset for Kubernetes")

	// Create the clientset for Metrics
	metricsClientset, err := versioned.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, errCreateMetricsClientset)
	}
	logger.Debug("Created clientset for Metrics")

	ctx := context.Background()

	pods, err := k8sClientset.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{})

	if err != nil {
		return errors.Wrap(err, errFetchAllPods)
	}

	crossplanePods := getCrossplanePods(pods.Items)
	logger.Debug("Fetched all Crossplane pods", "pods", crossplanePods, "namespace", c.Namespace)

	if len(crossplanePods) == 0 {
		fmt.Println("No Crossplane pods found in the namespace", c.Namespace)
		return nil
	}

	for i, pod := range crossplanePods {
		podMetrics, err := metricsClientset.MetricsV1beta1().PodMetricses(pod.PodNamespace).Get(ctx, pod.PodName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, errAddingPodMetrics)
		}
		for _, container := range podMetrics.Containers {
			if cpu := container.Usage.Cpu(); cpu != nil {
				crossplanePods[i].CPUUsage.Add(*cpu)
			}
			if memory := container.Usage.Memory(); memory != nil {
				crossplanePods[i].MemoryUsage.Add(*memory)
			}
		}
	}

	if err != nil {
		return errors.Wrap(err, errGetPodMetrics)
	}
	logger.Debug("Added metrics to Crossplane pods")

	sort.Slice(crossplanePods, func(i, j int) bool {
		if crossplanePods[i].PodType == crossplanePods[j].PodType {
			return crossplanePods[i].PodName < crossplanePods[j].PodName
		}
		return crossplanePods[i].PodType < crossplanePods[j].PodType
	})

	if c.Summary {
		printPodsSummary(k.Stdout, crossplanePods)
		logger.Debug("Printed pods summary")
		fmt.Println()
	}

	if err := printPodsTable(k.Stdout, crossplanePods); err != nil {
		return errors.Wrap(err, errPrintingPodsTable)
	}
	logger.Debug("Printed pods as table")
	return nil
}

func printPodsTable(w io.Writer, crossplanePods []topMetrics) error {
	tw := printers.GetNewTabWriter(w)
	// Building header
	headers := defaultPrinterRow{
		podType:   "TYPE",
		namespace: "NAMESPACE",
		name:      "NAME",
		cpu:       "CPU(cores)",
		memory:    "MEMORY",
	}
	_, err := fmt.Fprintln(tw, headers.String())
	if err != nil {
		return errors.Wrap(err, errWriteHeader)
	}

	// Building rows for each pod
	for _, pod := range crossplanePods {
		row := defaultPrinterRow{
			podType:   pod.PodType,
			namespace: pod.PodNamespace,
			name:      pod.PodName,
			// NOTE(phisco): inspired by https://github.com/kubernetes/kubectl/blob/97bd96adbceb24fd598bdc698da8794cb0b88e3b/pkg/metricsutil/metrics_printer.go#L209C6-L209C30
			cpu:    fmt.Sprintf("%vm", pod.CPUUsage.MilliValue()),
			memory: fmt.Sprintf("%vMi", pod.MemoryUsage.Value()/(1024*1024)),
		}
		_, err := fmt.Fprintln(tw, row.String())
		if err != nil {
			return errors.Wrap(err, errWriteRow)
		}
	}

	return tw.Flush()
}

func printPodsSummary(w io.Writer, pods []topMetrics) {
	categoryCounts := make(map[string]int)
	var totalMemoryUsage, totalCPUUsage resource.Quantity

	for _, pod := range pods {
		// Increment the count for this pod's category
		categoryCounts[pod.PodType]++

		// Aggregate CPU and Memory usage
		totalCPUUsage.Add(pod.CPUUsage)
		totalMemoryUsage.Add(pod.MemoryUsage)
	}

	// Print summary directly to the provided writer
	fmt.Fprintf(w, "Nr of Crossplane pods: %d\n", len(pods))
	// Sort categories alphabetically to ensure consistent output
	categories := make([]string, 0, len(categoryCounts))
	for category := range categoryCounts {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		fmt.Fprintf(w, "%s: %d\n", capitalizeFirst(category), categoryCounts[category])
	}
	fmt.Fprintf(w, "Memory: %s\n", fmt.Sprintf("%vMi", totalMemoryUsage.Value()/(1024*1024)))
	fmt.Fprintf(w, "CPU(cores): %s\n", fmt.Sprintf("%vm", totalCPUUsage.MilliValue()))
}

func getCrossplanePods(pods []v1.Pod) []topMetrics {
	metricsList := make([]topMetrics, 0)
	for _, pod := range pods {
		labels := pod.GetLabels()

		var podType string
		isCrossplanePod := false
		for labelKey, labelValue := range labels {
			switch {
			case strings.HasPrefix(labelKey, "pkg.crossplane.io/"):
				podType = strings.SplitN(labelKey, "/", 2)[1]
				if podType != "revision" {
					isCrossplanePod = true
				}
			case labelKey == "app.kubernetes.io/part-of" && labelValue == "crossplane":
				podType = "crossplane"
				isCrossplanePod = true
			}
			if isCrossplanePod {
				break
			}
		}

		if isCrossplanePod {
			metricsList = append(metricsList, topMetrics{
				PodType:      podType,
				PodName:      pod.Name,
				PodNamespace: pod.Namespace,
			})
		}
	}
	return metricsList
}

func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
