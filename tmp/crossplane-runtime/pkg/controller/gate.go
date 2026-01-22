package controller

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// A Gate is an interface to allow reconcilers to delay a callback until a set of GVKs are set to true inside the gate.
type Gate interface {
	// Register to call a callback function when all given GVKs are marked true. If the callback is unblocked, the
	// registration is removed.
	Register(callback func(), gvks ...schema.GroupVersionKind)
	// Set marks the associated condition to the given value. If the condition is already set as
	// that value, then this is a no-op. Returns true if there was an update detected.
	Set(gvk schema.GroupVersionKind, ready bool) bool
}
