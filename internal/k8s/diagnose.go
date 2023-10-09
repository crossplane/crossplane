package k8s

import (
	"reflect"
)

// Takes a Resource as input and returns a Resource containing all unhealthy Resources.
func Diagnose(r Resource, ur Resource) (Resource, error) {
	// Diagnose self
	if r.GetConditionStatus("Synced") == "False" || r.GetConditionStatus("Ready") == "False" {
		// If first resource is added to unhealthy Resource struct set it as root. Else add resource as child.
		if reflect.DeepEqual(ur, Resource{}) {
			// Dont add children, they have to be health checked first
			ur.Manifest = r.Manifest
			ur.Event = r.Event
		} else {
			// Dont add children, they have to be health checked first
			ur.Children = append(ur.Children, Resource{Manifest: r.Manifest, Event: r.Event})
		}
	}
	// Diagnose children
	for _, resource := range r.Children {
		ur, _ = Diagnose(resource, ur)
	}

	return ur, nil
}
