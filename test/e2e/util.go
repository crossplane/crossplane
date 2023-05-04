package main

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type scenarioContextKey struct{}

type resourceRef struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
}

// ScenarioContext keeps test scenario context
type ScenarioContext struct {
	Cluster                   *cluster
	Namespace                 string
	Claim                     *unstructured.Unstructured
	ClaimCompositeResourceRef *resourceRef
	Configuration             *unstructured.Unstructured
}

// ScenarioContextValue returns test context
func ScenarioContextValue(ctx context.Context) *ScenarioContext {
	return ctx.Value(scenarioContextKey{}).(*ScenarioContext)
}

// ToUnstructured returns Unstructured instance from its yaml string representation
func ToUnstructured(yamlContent string) (*unstructured.Unstructured, error) {
	m := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yamlContent), &m)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: m}, nil
}

func (ref *resourceRef) Type() string {
	return fmt.Sprintf("%s.%s", strings.ToLower(ref.Kind), strings.Split(ref.APIVersion, "/")[0])
}
