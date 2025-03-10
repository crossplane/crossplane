package diffprocessor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/beta/validate"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"io"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"strings"
)

// DiffProcessor handles the processing of resources for diffing.
type DiffProcessor struct {
	client    cc.ClusterClient
	config    *rest.Config
	namespace string
}

func NewDiffProcessor(config *rest.Config, client cc.ClusterClient, namespace string) (*DiffProcessor, error) {
	return &DiffProcessor{
		client:    client,
		config:    config,
		namespace: namespace,
	}, nil
}

// ProcessAll handles all resources stored in the processor.
func (p *DiffProcessor) ProcessAll(ctx context.Context, resources []*unstructured.Unstructured) error {
	var errs []error
	for _, res := range resources {
		if err := p.ProcessResource(ctx, res); err != nil {
			errs = append(errs, errors.Wrapf(err, "unable to process resource %s", res.GetName()))
		}
	}

	return errors.Join(errs...)
}

// ProcessResource handles one resource at a time.
func (p *DiffProcessor) ProcessResource(ctx context.Context, res *unstructured.Unstructured) error {
	comp, err := p.client.FindMatchingComposition(res)
	if err != nil {
		return errors.Wrap(err, "cannot find matching composition")
	}

	gvrs, selectors, err := p.IdentifyNeededExtraResources(comp)
	if err != nil {
		return errors.Wrap(err, "cannot identify needed extra resources")
	}

	extraResources, err := p.client.GetExtraResources(ctx, gvrs, selectors)
	if err != nil {
		return errors.Wrap(err, "cannot get extra resources")
	}

	fns, err := p.client.GetFunctionsFromPipeline(comp)
	if err != nil {
		return errors.Wrap(err, "cannot get functions from pipeline")
	}

	xr := ucomposite.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), xr); err != nil {
		return errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	hasTemplatedExtra, err := ScanForTemplatedExtraResources(comp)
	if err != nil {
		return errors.Wrap(err, "cannot scan for templated extra resources")
	}

	if hasTemplatedExtra {
		extraResources, err = p.HandleTemplatedExtraResources(ctx, comp, xr, fns, extraResources)
		if err != nil {
			return err
		}
	}

	desired, err := render.Render(ctx, nil, render.Inputs{
		CompositeResource: xr,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    extraResources,
	})
	if err != nil {
		return errors.Wrap(err, "cannot render resources")
	}

	xrdSchema, err := p.client.GetXRDSchema(ctx, res)
	if err != nil {
		return errors.Wrap(err, "cannot get XRD xrdSchema")
	}

	if err := ValidateResources(desired, xrdSchema); err != nil {
		return errors.Wrap(err, "cannot validate resources")
	}

	for _, d := range desired.ComposedResources {
		diff, err := CalculateDiff(p.config, &d)
		if err != nil {
			return errors.Wrap(err, "cannot calculate diff")
		}
		if diff != "" {
			fmt.Printf("%s\n---\n", diff)
		}
	}
	return nil
}

// IdentifyNeededExtraResources analyzes a composition to determine what extra resources are needed
func (p *DiffProcessor) IdentifyNeededExtraResources(comp *apiextensionsv1.Composition) ([]schema.GroupVersionResource, []metav1.LabelSelector, error) {
	// If no pipeline mode or no steps, return empty
	if comp.Spec.Mode == nil || *comp.Spec.Mode != apiextensionsv1.CompositionModePipeline {
		return nil, nil, nil
	}

	var resources []schema.GroupVersionResource
	var selectors []metav1.LabelSelector

	// Look for function-extra-resources steps
	for _, step := range comp.Spec.Pipeline {
		if step.FunctionRef.Name != "function-extra-resources" || step.Input == nil {
			continue
		}

		// Parse the input into an unstructured object
		input := &unstructured.Unstructured{}
		if err := input.UnmarshalJSON(step.Input.Raw); err != nil {
			return nil, nil, errors.Wrap(err, "cannot unmarshal function-extra-resources input")
		}

		// Extract extra resources configuration
		extraResources, found, err := unstructured.NestedSlice(input.Object, "spec", "extraResources")
		if err != nil || !found {
			continue
		}

		// Process each extra resource configuration
		for _, er := range extraResources {
			erMap, ok := er.(map[string]interface{})
			if !ok {
				continue
			}

			// Get the resource selector details
			apiVersion, _, _ := unstructured.NestedString(erMap, "apiVersion")
			kind, _, _ := unstructured.NestedString(erMap, "kind")
			selector, _, _ := unstructured.NestedMap(erMap, "selector", "matchLabels")

			if apiVersion == "" || kind == "" {
				continue
			}

			// Create GVR for this resource type
			gv, err := schema.ParseGroupVersion(apiVersion)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "cannot parse group version %q", apiVersion)
			}

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: fmt.Sprintf("%ss", strings.ToLower(kind)), // naive pluralization
			}
			resources = append(resources, gvr)

			// Create label selector
			labelSelector := metav1.LabelSelector{
				MatchLabels: make(map[string]string),
			}
			for k, v := range selector {
				if s, ok := v.(string); ok {
					labelSelector.MatchLabels[k] = s
				}
			}
			selectors = append(selectors, labelSelector)
		}
	}

	return resources, selectors, nil
}

// HandleTemplatedExtraResources processes templated extra resources.
func (p *DiffProcessor) HandleTemplatedExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *ucomposite.Unstructured, fns []pkgv1.Function, extraResources []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	preliminary, err := render.Render(ctx, nil, render.Inputs{
		CompositeResource: xr,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    extraResources,
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot perform preliminary render")
	}

	for _, result := range preliminary.Results {
		if result.GetKind() == "ExtraResources" {
			additional, err := GetExtraResourcesFromResult(&result)
			if err != nil {
				return nil, errors.Wrap(err, "cannot get extra resources from result")
			}
			extraResources = append(extraResources, additional...)
		}
	}

	return extraResources, nil
}

func ValidateResources(desired render.Outputs, schema *apiextensionsv1.CompositeResourceDefinition) error {
	// Convert XRD to CRD format
	crd := &extv1.CustomResourceDefinition{
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: schema.Spec.Group,
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     schema.Spec.Names.Kind,
				ListKind: schema.Spec.Names.ListKind,
				Plural:   schema.Spec.Names.Plural,
				Singular: schema.Spec.Names.Singular,
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    schema.Spec.Versions[len(schema.Spec.Versions)-1].Name,
					Served:  true,
					Storage: true,
					Schema:  &extv1.CustomResourceValidation{},
				},
			},
		},
	}

	// Convert the schema using JSON marshaling/unmarshaling
	raw, err := schema.Spec.Versions[len(schema.Spec.Versions)-1].Schema.OpenAPIV3Schema.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "cannot marshal XRD schema")
	}

	if err := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Unmarshal(raw); err != nil {
		return errors.Wrap(err, "cannot unmarshal CRD schema")
	}

	// Convert XR and composed resources to unstructured
	resources := make([]*unstructured.Unstructured, 0, len(desired.ComposedResources)+1)

	// Convert XR from composite.Unstructured to regular Unstructured
	xr := &unstructured.Unstructured{Object: desired.CompositeResource.UnstructuredContent()}
	resources = append(resources, xr)

	// Add composed resources to validation list
	for i := range desired.ComposedResources {
		resources = append(resources, &unstructured.Unstructured{Object: desired.ComposedResources[i].UnstructuredContent()})
	}

	// Validate using the converted CRD schema
	if err := validate.SchemaValidation(resources, []*extv1.CustomResourceDefinition{crd}, true, io.Discard); err != nil {
		return errors.Wrap(err, "schema validation failed")
	}

	return nil
}

func ScanForTemplatedExtraResources(comp *apiextensionsv1.Composition) (bool, error) {
	if comp.Spec.Mode == nil || *comp.Spec.Mode != apiextensionsv1.CompositionModePipeline {
		return false, nil
	}

	for _, step := range comp.Spec.Pipeline {
		if step.FunctionRef.Name != "function-go-templating" || step.Input == nil {
			continue
		}

		// Parse the input into an unstructured object
		input := &unstructured.Unstructured{}
		if err := input.UnmarshalJSON(step.Input.Raw); err != nil {
			return false, errors.Wrap(err, "cannot unmarshal function-go-templating input")
		}

		// Look for template string
		template, found, err := unstructured.NestedString(input.Object, "spec", "inline", "template")
		if err != nil || !found {
			continue
		}

		// Parse the template string as YAML and look for ExtraResources documents
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(template), 4096)
		for {
			obj := make(map[string]interface{})
			if err := decoder.Decode(&obj); err != nil {
				if err == io.EOF {
					break
				}
				return false, errors.Wrap(err, "cannot decode template YAML")
			}

			u := &unstructured.Unstructured{Object: obj}
			if u.GetKind() == "ExtraResources" {
				return true, nil
			}
		}
	}

	return false, nil
}

func GetExtraResourcesFromResult(result *unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	spec, found, err := unstructured.NestedMap(result.Object, "spec")
	if err != nil || !found {
		return nil, errors.New("no spec found in ExtraResources result")
	}

	extraResources, found, err := unstructured.NestedSlice(spec, "resources")
	if err != nil || !found {
		return nil, errors.New("no resources found in ExtraResources spec")
	}

	var resources []unstructured.Unstructured
	for _, er := range extraResources {
		erMap, ok := er.(map[string]interface{})
		if !ok {
			continue
		}

		u := unstructured.Unstructured{Object: erMap}
		resources = append(resources, u)
	}

	return resources, nil
}

func CalculateDiff(config *rest.Config, desired runtime.Object) (string, error) {
	// Get the current object from the cluster
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return "", errors.Wrap(err, "cannot create dynamic client")
	}

	// Convert desired to unstructured
	desiredUnstr, ok := desired.(*unstructured.Unstructured)
	if !ok {
		return "", errors.New("desired object is not unstructured")
	}

	// Create GVR from the object
	gvk := desiredUnstr.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: fmt.Sprintf("%ss", strings.ToLower(gvk.Kind)), // naive pluralization
	}

	// Get the current object
	current, err := dynamicClient.Resource(gvr).Namespace(desiredUnstr.GetNamespace()).Get(context.TODO(), desiredUnstr.GetName(), metav1.GetOptions{})
	if err != nil {
		// If not found, show the entire object as new
		if apierrors.IsNotFound(err) {
			b, err := json.MarshalIndent(desiredUnstr, "", "  ")
			if err != nil {
				return "", errors.Wrap(err, "cannot marshal desired object")
			}
			return fmt.Sprintf("+ %s (new object)\n%s", gvk.Kind, string(b)), nil
		}
		return "", errors.Wrap(err, "cannot get current object")
	}

	// Convert maps to JSON bytes for diffing
	currentBytes, err := json.Marshal(current.UnstructuredContent())
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal current object")
	}

	desiredBytes, err := json.Marshal(desiredUnstr.UnstructuredContent())
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal desired object")
	}

	// Calculate diff using strategic merge patch
	diff, err := strategicpatch.CreateTwoWayMergePatch(currentBytes, desiredBytes, desiredUnstr)
	if err != nil {
		return "", errors.Wrap(err, "cannot create patch")
	}

	// If there are no changes, return empty string
	if string(diff) == "{}" {
		return "", nil
	}

	// Format the diff
	var prettyDiff bytes.Buffer
	if err := json.Indent(&prettyDiff, diff, "", "  "); err != nil {
		return "", errors.Wrap(err, "cannot format diff")
	}

	return fmt.Sprintf("~ %s/%s\n%s", gvk.Kind, desiredUnstr.GetName(), prettyDiff.String()), nil
}
