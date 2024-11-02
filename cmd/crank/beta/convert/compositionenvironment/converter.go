package compositionenvironment

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// ConvertToFunctionEnvironmentConfigs converts a Composition to use function-environment-configs.
func ConvertToFunctionEnvironmentConfigs(in *unstructured.Unstructured, functionName string) (*unstructured.Unstructured, error) {
	if in == nil {
		return nil, errors.New("input is nil")
	}

	gvk := in.GetObjectKind().GroupVersionKind()

	if gvk.Empty() {
		return nil, errors.New("GroupVersionKind is empty")
	}

	if gvk.Group != v1.Group {
		return nil, errors.Errorf("GroupVersionKind Group is not %s", v1.Group)
	}

	if gvk.Kind != v1.CompositionKind {
		return nil, errors.Errorf("GroupVersionKind Kind is not %s", v1.CompositionKind)
	}

	out, err := fieldpath.PaveObject(in)
	if err != nil {
		return nil, err
	}

	if mode, err := out.GetString("spec.mode"); fieldpath.IsNotFound(err) || mode != string(v1.CompositionModePipeline) {
		return nil, errors.New("Composition is using Resources mode, run pipeline-composition command instead")
	}

	// Prepare function-environment-configs input
	inputPaved := fieldpath.Pave(map[string]any{
		"apiVersion": "environmentconfigs.fn.crossplane.io/v1beta1",
		"kind":       "Input",
	})

	var modified bool

	// Copy spec.environment.defaultData to function-environment-configs, if any
	if dd, err := out.GetValue("spec.environment.defaultData"); err == nil {
		if err := inputPaved.SetValue("spec.defaultData", dd); err != nil {
			return nil, errors.Wrap(err, "failed to set defaultData")
		}
		modified = true
	}

	// Copy spec.environment.environmentConfigs to function-environment-configs, if any
	if ec, err := out.GetValue("spec.environment.environmentConfigs"); err == nil {
		if err := inputPaved.SetValue("spec.environmentConfigs", ec); err != nil {
			return nil, errors.Wrap(err, "failed to set environmentConfigs")
		}
		modified = true
	}

	// Copy spec.environment.policy.resolution to function-environment-configs, if any
	if resolutionPolicy, err := out.GetString("spec.environment.policy.resolution"); err == nil {
		if err := inputPaved.SetValue("spec.policy.resolution", resolutionPolicy); err != nil {
			return nil, errors.Wrap(err, "failed to set policy.resolution")
		}
		modified = true
	}

	if !modified {
		// Nothing to do
		return nil, nil
	}

	// Nothing else should be left, we can delete the environment field
	if err := out.DeleteField("spec.environment"); err != nil {
		return nil, errors.Wrap(err, "failed to delete environment")
	}

	// Add function-environment-configs to the pipeline
	var pipeline []map[string]any
	if err := out.GetValueInto("spec.pipeline", &pipeline); err != nil {
		return nil, errors.Wrap(err, "failed to get pipeline")
	}

	if functionName == "" {
		functionName = "function-environment-configs"
	}

	pipeline = append([]map[string]any{
		{
			"step": "environment-configs",
			"functionRef": map[string]any{
				"name": functionName,
			},
			"input": inputPaved.UnstructuredContent(),
		},
	}, pipeline...)

	if err := out.SetValue("spec.pipeline", pipeline); err != nil {
		return nil, errors.Wrap(err, "failed to set pipeline")
	}

	return &unstructured.Unstructured{Object: out.UnstructuredContent()}, nil
}
