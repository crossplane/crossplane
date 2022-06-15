package claim

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	gotemplate "text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CompositeNameGenerator specifies the interface for a composite name
// generator.
type CompositeNameGenerator interface {
	// Generate generates a name for cp.
	Generate(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
}

var _ CompositeNameGenerator = &KubeAPINameGenerator{}

// KubeAPINameGenerator is a name generator that lets the kube API generate a
// name for a composite resource.
type KubeAPINameGenerator struct {
	client client.Client
}

// NewKubeAPINameGenerator creates a new KubeAPINameGenerator.
func NewKubeAPINameGenerator(client client.Client) *KubeAPINameGenerator {
	return &KubeAPINameGenerator{
		client: client,
	}
}

// Generate a composite name for the given claim.
func (k *KubeAPINameGenerator) Generate(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	// The API server returns an available name derived from
	// generateName when we perform a dry-run create. This name is
	// likely (but not guaranteed) to be available when we create
	// the composite resource. If the API server generates a name
	// that is unavailable it will return a 500 ServerTimeout error.
	cp.SetGenerateName(fmt.Sprintf("%s-", cm.GetName()))
	return k.client.Create(ctx, cp, client.DryRunAll)
}

var _ CompositeNameGenerator = &TemplateNameGenerator{}

// TemplateNameGenerator generates a composite name from a go template.
type TemplateNameGenerator struct {
	template *gotemplate.Template
}

// NewTemplateNameGenerator creates a new TemplateNameGenerator from the given
// go template.
func NewTemplateNameGenerator(template string) (*TemplateNameGenerator, error) {
	temp, err := gotemplate.
		New("claim-template").
		Funcs(sprig.HermeticTxtFuncMap()).
		Parse(template)
	if err != nil {
		return nil, err
	}
	return &TemplateNameGenerator{template: temp}, err
}

// Generate a composite name for the given claim.
func (t *TemplateNameGenerator) Generate(_ context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	data := map[string]interface{}{
		"Claim": map[string]interface{}{
			"Name":      cm.GetName(),
			"Namespace": cm.GetNamespace(),
			"Kind":      cm.GetObjectKind().GroupVersionKind().Kind,
			"Group":     cm.GetObjectKind().GroupVersionKind().Group,
			"Version":   cm.GetObjectKind().GroupVersionKind().Version,
			"GVK":       cm.GetObjectKind().GroupVersionKind().String(),
		},
	}
	buf := &bytes.Buffer{}
	if err := t.template.Execute(buf, data); err != nil {
		return err
	}
	name := strings.ToLower(buf.String())
	cp.SetName(name)
	return nil
}
