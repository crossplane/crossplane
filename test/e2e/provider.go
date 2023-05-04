package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/containers/image/docker/reference"
	"k8s.io/apimachinery/pkg/util/wait"
)

type providerInfo struct {
	Name     string
	ImageRef string
}

type providerCfgFunc func(info *providerInfo, ctx *ScenarioContext) error

var (
	providerTemplate = `
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: {{.Name}}
spec:
  package: {{.ImageRef}}
`

	providerType = "providers.pkg.crossplane.io"

	providerConfiguration = map[string]providerCfgFunc{
		"provider-dummy": providerDummyCfg,
	}
)

func providerGetsInstalled(ctx context.Context, providerImageRef string) error {
	ref, err := reference.ParseDockerRef(providerImageRef)
	if err != nil {
		return err
	}
	namedRef, ok := ref.(reference.NamedTagged)
	if !ok {
		return fmt.Errorf("%s has not tag", providerImageRef)
	}
	parts := strings.Split(reference.Path(namedRef), "/")
	providerName := parts[len(parts)-1]
	t, err := template.New("providerTemplate").Parse(providerTemplate)
	if err != nil {
		return err
	}
	rawYaml := &strings.Builder{}
	info := &providerInfo{Name: providerName, ImageRef: providerImageRef}
	if err = t.Execute(rawYaml, info); err != nil {
		return err
	}
	sc := ScenarioContextValue(ctx)
	c := sc.Cluster
	if err = c.ApplyYaml(rawYaml.String()); err != nil {
		return err
	}
	if err = providerMarkedAsInstalledAndHealthy(ctx, providerName); err != nil {
		return err
	}
	if cf, ok := providerConfiguration[providerName]; ok {
		if err = cf(info, sc); err != nil {
			return err
		}
	}
	return nil
}

func providerDummyCfg(_ *providerInfo, ctx *ScenarioContext) error {
	err := ctx.Cluster.ApplyYaml(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: server-dummy
  namespace: crossplane-system
  labels:
    app: server-dummy
spec:
  replicas: 1
  selector:
      matchLabels:
        app: server-dummy
  template:
    metadata:
      labels:
        app: server-dummy
    spec:
      containers:
        - name: server
          image: ghcr.io/upbound/provider-dummy-server:main
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 9090
`)
	if err != nil {
		return err
	}
	err = ctx.Cluster.ApplyYaml(`
apiVersion: v1
kind: Service
metadata:
  namespace: crossplane-system
  name: server-dummy
spec:
  selector:
      app: server-dummy
  ports:
    - port: 80
      targetPort: 9090
      protocol: TCP
`)
	if err != nil {
		return err
	}
	err = ctx.Cluster.ApplyYaml(`
apiVersion: dummy.upbound.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  endpoint: http://server-dummy.crossplane-system.svc.cluster.local
`)
	if err != nil {
		return err
	}
	return ctx.Cluster.WaitForResourceConditionMatch("deployment", "server-dummy", "crossplane-system", map[string]string{"Available": "True"})
}

func providerMarkedAsInstalledAndHealthy(ctx context.Context, name string) error {
	sc := ScenarioContextValue(ctx)
	return sc.Cluster.WaitForResourceConditionMatch(providerType, name, "default", map[string]string{"Healthy": "True", "Installed": "True"})
}

func providerNotInstalled(ctx context.Context, name string) error {
	sc := ScenarioContextValue(ctx)
	err := wait.PollImmediate(5*time.Second, 30*time.Second, func() (done bool, err error) {
		_, er2 := sc.Cluster.GetAndFilterResourceByJq(providerType, name, "default", ".")
		if er2 == nil {
			return true, fmt.Errorf("%s should not be installed", name)
		}
		return false, nil
	})
	if errors.Is(err, wait.ErrWaitTimeout) {
		return nil
	}
	return err
}
