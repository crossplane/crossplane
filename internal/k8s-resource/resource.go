package k8s_resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Resource struct {
	manifest *unstructured.Unstructured
	children []Resource
	event    string
}

// Returns resource kind as string
func (r Resource) GetKind() string {
	return r.manifest.GetKind()
}

// Returns resource name as string
func (r Resource) GetName() string {
	return r.manifest.GetName()
}

// Returns resource namespace as string
func (r Resource) GetNamespace() string {
	return r.manifest.GetNamespace()
}

// Returns resource apiversion as string
func (r Resource) GetApiVersion() string {
	return r.manifest.GetAPIVersion()
}

// This function takes a certain conditionType as input e.g. "Ready" or "Synced"
// Returns the Status of the map with the conditionType as string
func (r Resource) GetConditionStatus(conditionKey string) string {
	conditions, _, _ := unstructured.NestedSlice(r.manifest.Object, "status", "conditions")
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

// Returns the message as string if one is set under `status.conditions` in the manifest.
func (r Resource) GetConditionMessage() string {
	conditions, _, _ := unstructured.NestedSlice(r.manifest.Object, "status", "conditions")

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
	return r.event
}

// Returns true if the Resource has children set.
func (r Resource) GotChildren() bool {
	if len(r.children) > 0 {
		return true
	} else {
		return false
	}
}
