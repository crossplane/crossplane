/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package xrm

import (
	"context"
	"sort"
	"sync"

	v1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

// channelCapacity is the buffer size of the processing channel, should be a high value
// so that there is no blocking. Correctness of processing does not depend on the channel capacity.
var channelCapacity = 1000 //nolint:gochecknoglobals // we treat this as constant only overrideable for tests.

// workItem maintains the relationship of a resource to be loaded with its parent
// such that the resource that is loaded can be added as a child.
type workItem struct {
	parent *resource.Resource
	child  v1.ObjectReference
}

// resourceLoader is a delegate that loads resources and returns child resource refs.
type resourceLoader interface {
	loadResource(ctx context.Context, ref *v1.ObjectReference) *resource.Resource
	getResourceChildrenRefs(_ context.Context, r *resource.Resource) []v1.ObjectReference
}

// loader loads resources concurrently.
type loader struct {
	root         *resource.Resource // the root resource for which the tree is loaded
	l            resourceLoader     // the resource loader
	resourceLock sync.Mutex         // lock when updating the children of any resource
	processing   sync.WaitGroup     // "counter" to track requests in flight
	ch           chan workItem      // processing channel
	done         chan struct{}      // done channel, signaled when all resources are loaded
}

// newLoader creates a loader for the root resource.
func newLoader(root *resource.Resource, rl resourceLoader) *loader {
	l := &loader{
		l:    rl,
		ch:   make(chan workItem, channelCapacity),
		done: make(chan struct{}),
		root: root,
	}
	return l
}

// load loads the full resource tree in a concurrent manner.
func (l *loader) load(ctx context.Context, concurrency int) {
	// make sure counters are incremented for root child refs before starting concurrent processing
	refs := l.l.getResourceChildrenRefs(ctx, l.root)
	l.addRefs(l.root, refs)

	// signal the done channel after all items are processed
	go func() {
		l.processing.Wait()
		close(l.done)
	}()

	if concurrency < 1 {
		concurrency = defaultConcurrency
	}
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-l.done:
					return
				case item := <-l.ch:
					l.processItem(ctx, item)
				}
			}
		}()
	}
	wg.Wait()
	sortRefs(l.root)
}

func sortRefs(root *resource.Resource) {
	for _, child := range root.Children {
		sortRefs(child)
	}
	sort.Slice(root.Children, func(i, j int) bool {
		l := root.Children[i].Unstructured
		r := root.Children[j].Unstructured
		return l.GetAPIVersion()+l.GetKind()+l.GetName() < r.GetAPIVersion()+r.GetKind()+r.GetName()
	})
}

// addRefs adds work items to the queue.
func (l *loader) addRefs(parent *resource.Resource, refs []v1.ObjectReference) {
	// ensure counters are updated synchronously
	l.processing.Add(len(refs))
	// free up the current processing routine even if the channel blocks.
	go func() {
		for _, ref := range refs {
			l.ch <- workItem{
				parent: parent,
				child:  ref,
			}
		}
	}()
}

// processItem processes a single work item in the queue and decrements the in-process counter
// after adding child references.
func (l *loader) processItem(ctx context.Context, item workItem) {
	defer l.processing.Done()
	res := l.l.loadResource(ctx, &item.child)
	refs := l.l.getResourceChildrenRefs(ctx, res)
	l.updateChild(item, res)
	l.addRefs(res, refs)
}

// updateChild adds the supplied child resource to its parent.
func (l *loader) updateChild(item workItem, res *resource.Resource) {
	l.resourceLock.Lock()
	item.parent.Children = append(item.parent.Children, res)
	l.resourceLock.Unlock()
}
