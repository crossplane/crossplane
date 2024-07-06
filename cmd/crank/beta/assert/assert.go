package assert

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	errWriteOutput = "cannot write output"
)

// Assert compares expected resources against actual resources and reports any differences.
// It returns an error if there's an issue writing output.
func Assert(expectedResources, actualResources []*unstructured.Unstructured, skipSuccessLogs bool, w io.Writer) error {

	failure, missing := 0, 0
	for _, exp := range expectedResources {
		id := resourceID(exp)
		actualResource, exists := findMatchingResource(exp, actualResources)
		if !exists {
			missing++
			if _, err := fmt.Fprintf(w, "[x] %s\n - resource is missing\n", id); err != nil {
				return errors.Wrap(err, errWriteOutput)
			}
			continue
		}

		if _, err := isSubset(exp, actualResource); err != nil {
			failure++
			if _, err := fmt.Fprintf(w, "[x] %s\n%s\n", id, err); err != nil {
				return errors.Wrap(err, errWriteOutput)
			}
			continue
		}

		if skipSuccessLogs {
			continue
		}

		if _, err := fmt.Fprintf(w, "[✓] %s asserted successfully\n", id); err != nil {
			return errors.Wrap(err, errWriteOutput)
		}
	}

	fmt.Fprintf(os.Stdout, "\nTotal %d resources: %d missing resources, %d success cases, %d failure cases\n", len(expectedResources), missing, len(expectedResources)-failure-missing, failure)

	return nil
}

// findMatchingResource finds a matching resource based on GVK and labels in a slice of unstructured.Unstructured.
// It returns the matching resource and a boolean indicating if a match was found.
func findMatchingResource(expected *unstructured.Unstructured, actuals []*unstructured.Unstructured) (*unstructured.Unstructured, bool) {
	expGVK := expected.GroupVersionKind()
	expName := expected.GetName()
	expectedLabels, labelsExists, _ := unstructured.NestedStringMap(expected.Object, "metadata", "labels")

	for _, act := range actuals {
		actGVK := act.GroupVersionKind()
		actName := act.GetName()
		actLabels, _, _ := unstructured.NestedStringMap(act.Object, "metadata", "labels")

		if !reflect.DeepEqual(expGVK, actGVK) {
			continue
		}

		if expName != "" && expName != actName {
			continue
		}

		if labelsExists && !labelsAreSubset(expectedLabels, actLabels) {
			continue
		}

		return act, true
	}
	return nil, false
}

// labelsAreSubset checks if expectedLabels is a subset of actualLabels.
func labelsAreSubset(expectedLabels, actualLabels map[string]string) bool {
	for key, expValue := range expectedLabels {
		if actValue, found := actualLabels[key]; !found || actValue != expValue {
			return false
		}
	}
	return true
}

// resourceID generates a unique identifier string for a resource.
// It includes the GroupVersionKind, Name, and Labels of the resource.
func resourceID(resource *unstructured.Unstructured) string {
	id := resource.GroupVersionKind().String()

	labels := make(map[string]string)
	if labelMap, found, err := unstructured.NestedStringMap(resource.Object, "metadata", "labels"); found && err == nil {
		labels = labelMap
	}

	// Add the resource name
	if name, found, _ := unstructured.NestedString(resource.Object, "metadata", "name"); found {
		id += fmt.Sprintf(", Name=%s", name)
	}

	// If labels are not empty, concatenate them to gvk string.
	if len(labels) > 0 {
		labelStrings := make([]string, 0, len(labels))
		for key, value := range labels {
			labelStrings = append(labelStrings, fmt.Sprintf("%s: %s", key, value))
		}
		sort.Strings(labelStrings) // Sort for consistent output
		id += fmt.Sprintf(", Labels=[%s]", strings.Join(labelStrings, ", "))
	}

	return id
}
