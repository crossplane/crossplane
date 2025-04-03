// Package kubernetes contains interfaces and implementations for clients that talk directly to Kubernetes
package kubernetes

// Clients is an aggregation of all of our Kubernetes clients, used to pass them as a bundle,
// typically for initialization where the consumer can select which ones they need.
type Clients struct {
	Apply    ApplyClient
	Resource ResourceClient
	Schema   SchemaClient
	Type     TypeConverter
}
