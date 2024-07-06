package assert

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SubsetError struct modified to hold multiple errors
type SubsetError struct {
	path    []string
	message string
	errors  []*SubsetError // Store nested errors
}

// Error method to format the error message, including nested errors
func (e *SubsetError) Error() string {
	if len(e.errors) > 0 {
		var nestedErrors []string
		for _, err := range e.errors {
			nestedErrors = append(nestedErrors, fmt.Sprintf(" - %s", err.Error()))
		}
		return fmt.Sprintf("\n%s", strings.Join(nestedErrors, "\n"))
	}

	return fmt.Sprintf("%s: %s", e.pathString(), e.message)
}

// isSubset checks if 'actual' is a subset of 'expected'. Both parameters are interface{} to handle both structured and unstructured data.
func isSubset(expected, actual interface{}) (bool, error) {
	var errors []*SubsetError

	expected, actual = normalizeData(expected), normalizeData(actual)

	compare(expected, actual, nil, &errors)
	if len(errors) > 0 {
		// Sort errors by path
		sort.Slice(errors, func(i, j int) bool {
			return errors[i].pathString() < errors[j].pathString()
		})

		var errMsg []string
		for _, err := range errors {
			errMsg = append(errMsg, fmt.Sprintf(" - %s", err.Error()))
		}
		return false, fmt.Errorf(strings.Join(errMsg, "\n"))
	}
	return true, nil
}

// compare recursively compares data between expected and actual.
func compare(expected, actual interface{}, path []string, errors *[]*SubsetError) {

	if reflect.DeepEqual(expected, actual) {
		return
	}

	if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
		*errors = append(*errors, &SubsetError{
			path:    path,
			message: fmt.Sprintf("type mismatch: expected %T, got %T", expected, actual),
		})
		return
	}

	switch exp := expected.(type) {
	case map[string]interface{}:
		act, ok := actual.(map[string]interface{})
		if !ok {
			*errors = append(*errors, &SubsetError{
				path:    path,
				message: "expected a map but found a different type",
			})
			return
		}

		for k, vExp := range exp {
			vAct, exists := act[k]
			if !exists {
				*errors = append(*errors, &SubsetError{
					path:    append(path, k),
					message: "key is missing from map",
				})
				continue
			}
			compare(vExp, vAct, append(path, k), errors)
		}
	case []interface{}:
		act, ok := actual.([]interface{})
		if !ok || len(exp) != len(act) {
			*errors = append(*errors, &SubsetError{
				path:    path,
				message: fmt.Sprintf("expected an array of length %d, but got an array of length %d", len(exp), len(act)),
			})
			return
		}

		for i, vExp := range exp {
			compare(vExp, act[i], append(path, fmt.Sprintf("[%d]", i)), errors)
		}
	default:
		*errors = append(*errors, &SubsetError{
			path:    path,
			message: fmt.Sprintf("value mismatch: expected %v, got %v", expected, actual),
		})

	}
}

// normalizeData converts *unstructured.Unstructured to their underlying map[string]interface{} if necessary
func normalizeData(obj interface{}) interface{} {
	if uns, ok := obj.(*unstructured.Unstructured); ok {
		return uns.Object
	}
	return obj
}

// pathString formats the path for the error message
func (e *SubsetError) pathString() string {
	if len(e.path) == 0 {
		return ""
	}

	path := e.path[0]
	for i := 1; i < len(e.path); i++ {
		path = fmt.Sprintf("%s.%s", path, e.path[i])
	}
	return path
}
