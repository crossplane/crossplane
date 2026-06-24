/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"sync"

	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// A RequirementsCache remembers the requirements a function last returned for a
// composite resource (XR). The FetchingFunctionRunner uses it to pre-satisfy a
// function's requirements on the first call of a reconcile, rather than always
// discovering them on a first call and confirming them on a second.
//
// A composition function must return its requirements on every call - it can't
// assume Crossplane remembers them. Functions therefore typically gate on their
// requirements: if Crossplane satisfied them, proceed; if not, return early
// asking for them to be satisfied. A function's requirements are usually stable
// across reconciles (e.g. derived from the XR), so remembering them lets us
// satisfy the gate on the first call. If our memory is wrong - the function
// returns different requirements than we remembered - the runner falls back to
// its iterative fetch loop and self-corrects.
//
// A RequirementsCache is safe for concurrent use.
type RequirementsCache struct {
	mx sync.RWMutex

	// Keyed by XR UID. We key by XR UID rather than name/namespace so that
	// recreating an XR with the same name doesn't inherit a stale predecessor's
	// requirements.
	entries map[string]*xrRequirements
}

// xrRequirements is what a RequirementsCache remembers for one XR: the
// requirements each of its functions returned, plus the XR's kind so the cache
// can evict entries for deleted XRs of a given kind.
type xrRequirements struct {
	gvk        schema.GroupVersionKind
	byFunction map[string]*fnv1.Requirements
}

// A RequirementsCache is a RequirementsRecorder.
var _ RequirementsRecorder = &RequirementsCache{}

// NewRequirementsCache creates a RequirementsCache.
func NewRequirementsCache() *RequirementsCache {
	return &RequirementsCache{entries: make(map[string]*xrRequirements)}
}

// A nopRequirementsRecorder remembers nothing. It's the FetchingFunctionRunner's
// default, so that by default the runner discovers a function's requirements
// afresh each reconcile rather than pre-satisfying remembered ones.
type nopRequirementsRecorder struct{}

func (nopRequirementsRecorder) Get(_, _ string) (*fnv1.Requirements, bool) { return nil, false }
func (nopRequirementsRecorder) Set(_ string, _ schema.GroupVersionKind, _ string, _ *fnv1.Requirements) {
}

// Get returns the requirements the named function last returned for the XR with
// the supplied UID, if any. It returns a clone, so callers can't mutate the
// cached copy, and so a caller holding the result doesn't race with a concurrent
// Set.
func (c *RequirementsCache) Get(xrUID, function string) (*fnv1.Requirements, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()

	e, ok := c.entries[xrUID]
	if !ok {
		return nil, false
	}
	r, ok := e.byFunction[function]
	if !ok {
		return nil, false
	}

	clone, ok := proto.Clone(r).(*fnv1.Requirements)
	if !ok {
		return nil, false
	}
	return clone, true
}

// RequiredGVKs returns the distinct GroupVersionKinds of the resources required
// by any of the XR's functions. The watch garbage collector uses it to learn
// which required resource watches are still in use.
func (c *RequirementsCache) RequiredGVKs(xrUID string) []schema.GroupVersionKind {
	c.mx.RLock()
	defer c.mx.RUnlock()

	e, ok := c.entries[xrUID]
	if !ok {
		return nil
	}

	seen := make(map[schema.GroupVersionKind]bool)
	gvks := make([]schema.GroupVersionKind, 0)
	for _, r := range e.byFunction {
		for _, gvk := range requiredGVKs(r) {
			if seen[gvk] {
				continue
			}
			seen[gvk] = true
			gvks = append(gvks, gvk)
		}
	}
	return gvks
}

// Set records the requirements the named function returned for the XR with the
// supplied UID and kind. It clones the requirements so the caller can't mutate
// the cached copy. Set deletes the function's entry when passed empty
// requirements, so an XR whose function stops requiring resources doesn't retain
// a stale entry.
func (c *RequirementsCache) Set(xrUID string, gvk schema.GroupVersionKind, function string, r *fnv1.Requirements) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if isEmptyRequirements(r) {
		if e, ok := c.entries[xrUID]; ok {
			delete(e.byFunction, function)
			if len(e.byFunction) == 0 {
				delete(c.entries, xrUID)
			}
		}
		return
	}

	e, ok := c.entries[xrUID]
	if !ok {
		e = &xrRequirements{gvk: gvk, byFunction: make(map[string]*fnv1.Requirements)}
		c.entries[xrUID] = e
	}

	// proto.Clone of a *fnv1.Requirements always returns a *fnv1.Requirements,
	// but we check the assertion to satisfy the linter and guard against a
	// future change to the message type.
	clone, ok := proto.Clone(r).(*fnv1.Requirements)
	if !ok {
		return
	}
	e.byFunction[function] = clone
}

// RetainForKind forgets the requirements of every XR of the supplied kind whose
// UID isn't in the supplied set of live UIDs. The watch garbage collector calls
// it with the UIDs of the XRs that still exist, so the cache doesn't leak
// entries for deleted XRs.
func (c *RequirementsCache) RetainForKind(gvk schema.GroupVersionKind, live map[string]bool) {
	c.mx.Lock()
	defer c.mx.Unlock()

	for uid, e := range c.entries {
		if e.gvk == gvk && !live[uid] {
			delete(c.entries, uid)
		}
	}
}

// isEmptyRequirements returns true if r requires nothing. We treat nil and a
// requirements message with no selectors identically - both mean "this function
// needs nothing", which we don't bother remembering.
func isEmptyRequirements(r *fnv1.Requirements) bool {
	if r == nil {
		return true
	}
	//nolint:staticcheck // We must account for the deprecated extra_resources field.
	return len(r.GetResources()) == 0 && len(r.GetExtraResources()) == 0 && len(r.GetSchemas()) == 0
}
