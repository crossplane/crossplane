package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Resource struct {
	Manifest *unstructured.Unstructured
	Children []Resource
	Event    string
}

// Returns resource kind as string
func (r Resource) GetKind() string {
	return r.Manifest.GetKind()
}

// Returns resource name as string
func (r Resource) GetName() string {
	return r.Manifest.GetName()
}

// Returns resource namespace as string
func (r Resource) GetNamespace() string {
	return r.Manifest.GetNamespace()
}

// Returns resource apiversion as string
func (r Resource) GetApiVersion() string {
	return r.Manifest.GetAPIVersion()
}

// This function takes a certain conditionType as input e.g. "Ready" or "Synced"
// Returns the Status of the map with the conditionType as string
func (r Resource) GetConditionStatus(conditionKey string) string {
	conditions, _, _ := unstructured.NestedSlice(r.Manifest.Object, "status", "conditions")
	for _, condition := range conditions {
		conditionMap, _ := condition.(map[string]interface{})
		conditionType, _ := conditionMap["type"].(string)
		conditionStatus, _ := conditionMap["status"].(string)

		if conditionType == conditionKey {
			return conditionStatus
		}
	}
	return ""
}

// Returns the message as string if set under `status.conditions` in the manifest. Else return empty string
func (r Resource) GetConditionMessage() string {
	conditions, _, _ := unstructured.NestedSlice(r.Manifest.Object, "status", "conditions")

	for _, item := range conditions {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if message, exists := itemMap["message"]; exists {
				if messageStr, ok := message.(string); ok {
					return messageStr
				}
			}
		}
	}

	return ""
}

// Returns the latest event of the resource as string
func (r Resource) GetEvent() string {
	return r.Event
}

// Returns true if the Resource has children set.
func (r Resource) GotChildren() bool {
	if len(r.Children) > 0 {
		return true
	} else {
		return false
	}
}
