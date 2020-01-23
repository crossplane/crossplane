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

package engines

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"

	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

// Helm2EngineRunner creates resources from files which are specified in the helm2 format
type Helm2EngineRunner struct {
	Log logr.Logger
}

const (
	spec             = "spec"
	valuesYaml       = "values.yaml"
	engineImageName  = "crossplane/resource-engine-helm2:master"
	kubectlImageName = "crossplane/resource-engine-kubectl:master"

	engineCfgVolumeName = "engine-configuration"
	engineCfgDir        = "/usr/share/engine-configuration/"

	stackVolumeName = "stack-configuration"
	stackDestDir    = "/usr/share/input/"

	resourceCfgVolumeName = "resource-configuration"
	resourceCfgDestDir    = "/usr/share/resource-configuration/"
)

var (
	// TODO this file name should not be hard-coded
	engineCfgFile = filepath.Join(engineCfgDir, valuesYaml)
)

// CreateConfig creates a config map to be consumed by an execution of the helm2 engine.
// When a behavior executes, the resource engine is configured by the
// object which triggered the behavior. This method encapsulates the logic to
// create the resource engine configuration from the object's fields.
// TODO it seems as though a lot of the transformation logic is probably reusable
func (her *Helm2EngineRunner) CreateConfig(claim *unstructured.Unstructured, hc *v1alpha1.HookConfiguration) (*corev1.ConfigMap, error) {
	// yamlyamlyamlyamlyaml
	// TODO if spec is missing, this won't work very well
	s, ok := claim.Object[spec]

	if !ok {
		her.Log.V(logging.Debug).Info("Spec not found on claim; not creating engine configuration", "claim", claim)
	}

	her.Log.V(logging.Debug).Info("Converting configuration", "spec", s)
	configContents, err := yaml.Marshal(s)

	her.Log.V(logging.Debug).Info("Configuration contents as yaml", "configContents", configContents)

	if err != nil {
		her.Log.Error(err, "Error marshaling claim spec as yaml!", "claim", claim)
		return nil, err
	}

	// Underneath, the yamler uses https://godoc.org/encoding/json#Marshal,
	// which means that the bytes are UTF-8 encoded
	// Theoretically we could get better performance by using a binary config
	// map, but having a string makes it better for humans who may want to observe
	// or troubleshoot behavior.
	stringConfigContents := string(configContents)

	// TODO get the engine type from the configuration
	engineType := hc.Engine.Type

	// TODO engine type should have a bit more structure;
	// probably better to use an enum type pattern, with an
	// engine name and its corresponding configuration file
	// name in the same object
	configKeyName := ""

	if engineType == Helm2EngineType {
		configKeyName = valuesYaml
	}

	configName := string(claim.GetUID())
	generatedMap, err := generateConfigMap(configName, configKeyName, stringConfigContents, her.Log)

	if err != nil {
		her.Log.Info("Error generating config map!", "claim", claim, "error", err)
		return nil, err
	}

	generatedMap.SetNamespace(claim.GetNamespace())

	her.Log.V(logging.Debug).Info("Generated config map to pass engine configuration", "configMap", generatedMap)

	return generatedMap, err
}

// RunEngine executes the helm2 engine to create some resources.
// TODO we could potentially have a method create the job, and a higher-level one execute it.
func (her *Helm2EngineRunner) RunEngine(ctx context.Context, client client.Client, claim *unstructured.Unstructured, config *corev1.ConfigMap, stackSource string, hc *v1alpha1.HookConfiguration) (*unstructured.Unstructured, error) {
	// TODO if there is no config specified, either use an empty config or don't specify
	// one at all.

	// TODO if we change this to meta.AsController, and we have the controller-runtime controller configured
	// to Own Jobs, then we'll get a reconcile call when Jobs finish. However, we'd need to change the logic
	// for the reconcile a bit to support that effectively. For example:
	// - We wouldn't want to create jobs every time reconcile is run
	//   * This means keeping track of created jobs somewhere and could also mean using deterministic job names
	ownerRef := meta.AsOwner(meta.ReferenceTo(claim, claim.GroupVersionKind()))
	var jobBackoff int32

	// TODO target stack image will come from the stack object, or maybe the stack install object.
	// Then for each resource behavior hook, we want to run the hook
	// TODO update this to use the most recent format, where a hook is a structured object

	resourceDir := fmt.Sprintf("/.registry/%s", hc.Directory)

	namespace := claim.GetNamespace()

	// TODO we should generate a name and save a reference on the claim
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "helm-template-apply-",
			Namespace:    namespace,
			OwnerReferences: []metav1.OwnerReference{
				ownerRef,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &jobBackoff,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					// Init containers guarantee that each one will complete successfully
					// before the next one starts. See this documentation for more details:
					// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#understanding-init-containers
					InitContainers: []corev1.Container{
						{
							Name:  "load-stack",
							Image: stackSource,
							Command: []string{
								// The "." suffix causes the cp -R to copy the contents of the directory instead of
								// the directory itself
								"cp", "-R", fmt.Sprintf("%s/.", resourceDir), stackDestDir,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      stackVolumeName,
									MountPath: stackDestDir,
								},
							},
						},
						{
							Name:  "engine",
							Image: engineImageName,
							Command: []string{
								"helm",
							},
							Args: []string{
								"template",
								"--output-dir", resourceCfgDestDir,
								"--namespace", namespace,
								"--values", engineCfgFile,
								stackDestDir,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      stackVolumeName,
									MountPath: stackDestDir,
								},
								{
									Name:      resourceCfgVolumeName,
									MountPath: resourceCfgDestDir,
								},
								{
									Name:      engineCfgVolumeName,
									MountPath: engineCfgDir,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "kubectl",
							Image: kubectlImageName,
							Command: []string{
								"kubectl",
							},
							Args: []string{
								"apply",
								"--namespace", namespace,
								"-R",
								"-f",
								resourceCfgDestDir,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      resourceCfgVolumeName,
									MountPath: resourceCfgDestDir,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: stackVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: resourceCfgVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: engineCfgVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: config.GetName(),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// TODO theoretically this won't be creating a job every time, and eventually we'll want to return a status or result of some sort
	// so that our shared reconciler logic can expose it, probably by updating the claim status.
	return nil, client.Create(ctx, job)
}

// NewHelm2EngineRunner is a convenience method to create a new Helm2EngineRunner.
func NewHelm2EngineRunner(log logr.Logger) *Helm2EngineRunner {
	return &Helm2EngineRunner{
		Log: log,
	}
}
