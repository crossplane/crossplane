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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/vladimirvivien/gexe"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

func TestFunctions(t *testing.T) {

	manifests := "test/e2e/manifests/apiextensions/composition/functions/private-registry"
	environment.Test(t,
		features.New("PullImageFromPrivateRegistryWithCustomCert").
			WithLabel(LabelArea, "xfn").
			Setup(funcs.CreateNamespace("reg")).
			Setup(CreateTLSCertificateAsSecret("private-docker-registry.reg.svc.cluster.local", "reg")).
			Setup(InstallDockerRegistry()).
			Setup(CopyImagesToRegistry()).
			Setup(CrossplaneDeployedWithFunctionsEnabled()).
			Setup(ProvideNopDeployed()).
			Assess("CompositionWithFunctionIsCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition.yaml"),
			)).
			Assess("ClaimIsCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimBecomesAvailable", funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("ManagedResourcesProcessedByFunction", ManagedResourcedProcessedByFunction()).
			Teardown(funcs.AsFeaturesFunc(funcs.HelmUpgrade(HelmOptions()...))).
			Feature(),
	)
}

func resourceGetter(ctx context.Context, t *testing.T, config *envconf.Config) func(string, string, string, string) *unstructured.Unstructured {
	return func(name string, namespace string, apiVersion string, kind string) *unstructured.Unstructured {
		client := config.Client().Resources().GetControllerRuntimeClient()
		u := &unstructured.Unstructured{}
		gv, err := schema.ParseGroupVersion(apiVersion)
		if err != nil {
			t.Fatal(err)
		}
		u.SetGroupVersionKind(gv.WithKind(kind))
		if err := client.Get(ctx, client2.ObjectKey{Name: name, Namespace: namespace}, u); err != nil {
			t.Fatal("cannot get claim", err)
		}
		return u
	}
}
func resourceValue(t *testing.T, u *unstructured.Unstructured, path ...string) map[string]string {
	f, found, err := unstructured.NestedStringMap(u.Object, path...)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("field not found at path %v", path)
	}
	return f
}

func resourceSliceValue(t *testing.T, u *unstructured.Unstructured, path ...string) []map[string]string {
	f, found, err := unstructured.NestedSlice(u.Object, path...)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("field not found at path %v", path)
	}
	var s []map[string]string
	for _, v := range f {
		if vv, ok := v.(map[string]interface{}); ok {
			s = append(s, asMapOfStrings(vv))
		} else {
			t.Fatalf("not a map[string]string: %v type %s", v, reflect.TypeOf(v))
		}
	}
	return s
}

func asMapOfStrings(m map[string]interface{}) map[string]string {
	r := make(map[string]string)
	for k, v := range m {
		r[k] = fmt.Sprintf("%v", v)
	}
	return r
}

// ManagedResourcedProcessedByFunction asserts that MRs contains the requested label
func ManagedResourcedProcessedByFunction() features.Func {

	return func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
		labelName := "labelizer.xfn.crossplane.io/processed"
		rg := resourceGetter(ctx, t, config)
		claim := rg("fn-labelizer", "default", "nop.example.org/v1alpha1", "NopResource")
		r := resourceValue(t, claim, "spec", "resourceRef")

		xr := rg(r["name"], "default", r["apiVersion"], r["kind"])
		mrefs := resourceSliceValue(t, xr, "spec", "resourceRefs")
		for _, mref := range mrefs {
			mr := rg(mref["name"], "default", mref["apiVersion"], mref["kind"])
			l, found := mr.GetLabels()[labelName]
			if !found {
				t.Fatalf("managed resource %v was not processed by function", mr)
			}
			if l != "true" {
				t.Fatalf("Expected label %v value to be true, but got %v", labelName, l)
			}
		}
		return ctx
	}
}

// CreateTLSCertificateAsSecret for given dns name and store the secret in the given namespace
func CreateTLSCertificateAsSecret(dnsName string, ns string) features.Func {
	return func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
		caPem, keyPem, err := createCert(dnsName)
		if err != nil {
			t.Fatal(err)
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "reg-cert",
				Namespace: ns,
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
	}
}

// CopyImagesToRegistry copies fn images to private registry
func CopyImagesToRegistry() features.Func {
	return func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
		pfw := gexe.StartProc("kubectl port-forward service/private-docker-registry -n reg 32000:5000")
		defer func() {
			pfw.Kill()
			pfw.Wait()
			out, _ := io.ReadAll(pfw.Out())
			t.Log("out", string(out), "err", pfw.Err())
		}()
		time.Sleep(20 * time.Second)
		_ = gexe.Run("docker tag crossplane-e2e/fn-labelizer:latest localhost:32000/fn-labelizer:latest")
		p := gexe.RunProc("docker push localhost:32000/fn-labelizer:latest")
		if p.ExitCode() != 0 {
			out, _ := io.ReadAll(p.Out())
			t.Fatalf("copying image to registry not successful, exit code %v std out %v std err %v", p.ExitCode(), string(out), p.Err())
		}
		return ctx
	}
}

// InstallDockerRegistry with custom TLS
func InstallDockerRegistry() features.Func {
	return funcs.AsFeaturesFunc(funcs.HelmInstall(
		helm.WithName("private"),
		helm.WithNamespace("reg"),
		helm.WithWait(),
		helm.WithChart("https://helm.twun.io/docker-registry-2.2.1.tgz"),
		helm.WithArgs(
			"--set service.type=NodePort",
			"--set service.nodePort=32000",
			"--set tlsSecretName=reg-cert",
		),
	))
}

// CrossplaneDeployedWithFunctionsEnabled asserts that crossplane deployment with composition functions is enabled
func CrossplaneDeployedWithFunctionsEnabled() features.Func {
	return funcs.AllOf(
		funcs.AsFeaturesFunc(funcs.HelmUpgrade(
			HelmOptions(
				helm.WithArgs(
					"--set args={--debug,--enable-composition-functions}",
					"--set xfn.enabled=true",
					"--set xfn.args={--debug}",
					"--set registryCaBundleConfig.name=reg-ca",
					"--set registryCaBundleConfig.key=domain.crt",
				))...)),
		funcs.ReadyToTestWithin(1*time.Minute, namespace),
	)
}

// ProvideNopDeployed assets that provider-nop is deployed and healthy
func ProvideNopDeployed() features.Func {
	manifests := "test/e2e/manifests/apiextensions/composition/minimal/prerequisites"
	return funcs.AllOf(
		funcs.ApplyResources(FieldManager, manifests, "provider.yaml"),
		funcs.ApplyResources(FieldManager, manifests, "definition.yaml"),
		funcs.ResourcesCreatedWithin(30*time.Second, manifests, "provider.yaml"),
		funcs.ResourcesCreatedWithin(30*time.Second, manifests, "definition.yaml"),
		funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "definition.yaml", v1.WatchingComposite()),
	)
}

func createCert(dnsName string) (string, string, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
			CommonName:    dnsName,
		},
		DNSNames:              []string{dnsName},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return "", "", err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	keyPEM := new(bytes.Buffer)
	pem.Encode(keyPEM, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	return caPEM.String(), keyPEM.String(), nil
}
