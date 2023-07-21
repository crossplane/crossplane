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
package e2e

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/test/e2e/funcs"
	"github.com/crossplane/crossplane/test/e2e/utils"
)

const (
	registryNs = "xfn-registry"

	timeoutFive = 5 * time.Minute
	timeoutOne  = 1 * time.Minute
)

func TestXfnRunnerImagePull(t *testing.T) {

	manifests := "test/e2e/manifests/xfnrunner/private-registry/pull"
	environment.Test(t,
		features.New("PullFnImageFromPrivateRegistryWithCustomCert").
			WithLabel(LabelArea, "xfn").
			WithSetup("InstallRegistryWithCustomTlsCertificate",
				funcs.AllOf(
					funcs.AsFeaturesFunc(envfuncs.CreateNamespace(registryNs)),
					func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
						dnsName := "private-docker-registry.xfn-registry.svc.cluster.local"
						caPem, keyPem, err := utils.CreateCert(dnsName)
						if err != nil {
							t.Fatal(err)
						}

						secret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "reg-cert",
								Namespace: registryNs,
							},
							Type: corev1.SecretTypeTLS,
							StringData: map[string]string{
								"tls.crt": caPem,
								"tls.key": keyPem,
							},
						}
						client := config.Client().Resources()
						if err := client.Create(ctx, secret); err != nil {
							t.Fatalf("Cannot create secret %s: %v", secret.Name, err)
						}
						configMap := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "reg-ca",
								Namespace: namespace,
							},
							Data: map[string]string{
								"domain.crt": caPem,
							},
						}
						if err := client.Create(ctx, configMap); err != nil {
							t.Fatalf("Cannot create config %s: %v", configMap.Name, err)
						}
						return ctx
					},

					funcs.AsFeaturesFunc(
						funcs.HelmRepo(
							helm.WithArgs("add"),
							helm.WithArgs("twuni"),
							helm.WithArgs("https://helm.twun.io"),
						)),
					funcs.AsFeaturesFunc(
						funcs.HelmInstall(
							helm.WithName("private"),
							helm.WithNamespace(registryNs),
							helm.WithWait(),
							helm.WithChart("twuni/docker-registry"),
							helm.WithVersion("2.2.2"),
							helm.WithArgs(
								"--set service.type=NodePort",
								"--set service.nodePort=32000",
								"--set tlsSecretName=reg-cert",
							),
						))),
			).
			WithSetup("CopyFnImageToRegistry",
				funcs.CopyImageToRegistry(clusterName, registryNs, "private-docker-registry", "crossplane-e2e/fn-labelizer:latest", timeoutOne)).
			WithSetup("CrossplaneDeployedWithFunctionsEnabled", funcs.AllOf(
				funcs.AsFeaturesFunc(funcs.HelmUpgrade(
					HelmOptions(
						helm.WithArgs(
							"--set args={--debug,--enable-composition-functions}",
							"--set xfn.enabled=true",
							"--set xfn.args={--debug}",
							"--set registryCaBundleConfig.name=reg-ca",
							"--set registryCaBundleConfig.key=domain.crt",
							"--set xfn.resources.requests.cpu=100m",
							"--set xfn.resources.limits.cpu=100m",
						),
						helm.WithWait())...)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("ProviderNopDeployed", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/provider.yaml"),
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/definition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/provider.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/definition.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", v1.WatchingComposite()),
			)).
			Assess("CompositionWithFunctionIsCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition.yaml"),
			)).
			Assess("ClaimIsCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimBecomesAvailable", funcs.ResourcesHaveConditionWithin(timeoutFive, manifests, "claim.yaml", xpv1.Available())).
			Assess("ManagedResourcesProcessedByFunction", funcs.ManagedResourcesOfClaimHaveFieldValueWithin(timeoutFive, manifests, "claim.yaml", "metadata.labels[labelizer.xfn.crossplane.io/processed]", "true", nil)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			WithTeardown("DeleteComposition", funcs.AllOf(
				funcs.DeleteResources(manifests, "composition.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "composition.yaml"),
			)).
			WithTeardown("ProviderNopRemoved", funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/provider.yaml"),
				funcs.DeleteResources(manifests, "prerequisites/definition.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "prerequisites/provider.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "prerequisites/definition.yaml"),
			)).
			WithTeardown("RemoveRegistry", funcs.AllOf(
				funcs.AsFeaturesFunc(envfuncs.DeleteNamespace(registryNs)),
				func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
					client := config.Client().Resources(namespace)
					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "reg-ca",
							Namespace: namespace,
						},
					}
					err := client.Delete(ctx, configMap)
					if err != nil {
						t.Fatal(err)
					}
					return ctx
				},
			)).
			WithTeardown("CrossplaneDeployedWithoutFunctionsEnabled", funcs.AllOf(
				funcs.AsFeaturesFunc(funcs.HelmUpgrade(HelmOptions()...)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestXfnRunnerWriteToTmp(t *testing.T) {
	manifests := "test/e2e/manifests/xfnrunner/tmp-writer"
	environment.Test(t,
		features.New("CreateAFileInTmpFolder").
			WithLabel(LabelArea, "xfn").
			WithSetup("InstallRegistry",
				funcs.AllOf(
					funcs.AsFeaturesFunc(envfuncs.CreateNamespace(registryNs)),
					funcs.AsFeaturesFunc(
						funcs.HelmRepo(
							helm.WithArgs("add"),
							helm.WithArgs("twuni"),
							helm.WithArgs("https://helm.twun.io"),
						)),
					funcs.AsFeaturesFunc(
						funcs.HelmInstall(
							helm.WithName("public"),
							helm.WithNamespace(registryNs),
							helm.WithWait(),
							helm.WithChart("twuni/docker-registry"),
							helm.WithVersion("2.2.2"),
							helm.WithArgs(
								"--set service.type=NodePort",
								"--set service.nodePort=32000",
							),
						))),
			).
			WithSetup("CopyFnImageToRegistry",
				funcs.CopyImageToRegistry(clusterName, registryNs, "public-docker-registry", "crossplane-e2e/fn-tmp-writer:latest", timeoutOne)).
			WithSetup("CrossplaneDeployedWithFunctionsEnabled", funcs.AllOf(
				funcs.AsFeaturesFunc(funcs.HelmUpgrade(
					HelmOptions(
						helm.WithArgs(
							"--set args={--debug,--enable-composition-functions}",
							"--set xfn.enabled=true",
							"--set xfn.args={--debug}",
							"--set xfn.resources.requests.cpu=100m",
							"--set xfn.resources.limits.cpu=100m",
						),
						helm.WithWait())...)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("ProviderNopDeployed", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/provider.yaml"),
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/definition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/provider.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/definition.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", v1.WatchingComposite()),
			)).
			Assess("CompositionWithFunctionIsCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition.yaml"),
			)).
			Assess("ClaimIsCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimBecomesAvailable",
				funcs.ResourcesHaveConditionWithin(timeoutFive, manifests, "claim.yaml", xpv1.Available())).
			Assess("ManagedResourcesProcessedByFunction",
				funcs.ManagedResourcesOfClaimHaveFieldValueWithin(timeoutFive, manifests, "claim.yaml", "metadata.labels[tmp-writer.xfn.crossplane.io]", "true",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			WithTeardown("DeleteComposition", funcs.AllOf(
				funcs.DeleteResources(manifests, "composition.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "composition.yaml"),
			)).
			WithTeardown("ProviderNopRemoved", funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/provider.yaml"),
				funcs.DeleteResources(manifests, "prerequisites/definition.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "prerequisites/provider.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "prerequisites/definition.yaml"),
			)).
			WithTeardown("RemoveRegistry", funcs.AsFeaturesFunc(envfuncs.DeleteNamespace(registryNs))).
			WithTeardown("CrossplaneDeployedWithoutFunctionsEnabled", funcs.AllOf(
				funcs.AsFeaturesFunc(funcs.HelmUpgrade(HelmOptions()...)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}
