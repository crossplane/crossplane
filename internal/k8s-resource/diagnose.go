package k8s_resource

import (
	"reflect"
)

// The Diagnose function takes a r Resource, which should contain at least one resource.
// The unhealthyR Resource is an initialy empty Resource which is used to store the identified unhealthy resources.
// The function then returns the unhealthyR
func Diagnose(r Resource, unhealthyR Resource) (Resource, error) {
	// Diagnose self
	if r.GetConditionStatus("Synced") == "False" || r.GetConditionStatus("Ready") == "False" {
		// If first resource is added to unhealthy Resource struct set it as root. Else resource as child.
		if reflect.DeepEqual(unhealthyR, Resource{}) {
			// Dont add children.
			unhealthyR.manifest = r.manifest
			unhealthyR.event = r.event
		} else {
			// Dont append children
			unhealthyR.children = append(unhealthyR.children, Resource{manifest: r.manifest, event: r.event})
		}
	}
	// Diagnose children
	for _, resource := range r.children {
		unhealthyR, _ = Diagnose(resource, unhealthyR)
	}

	return unhealthyR, nil
}
