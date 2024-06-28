package xrm

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

// simpleGenerator generates a tree of resources for a specific depth and the number of children to
// create at any level.
type simpleGenerator struct {
	childDepth int
	numItems   int
	l          sync.Mutex     // lock for accessing the depth map
	depthMap   map[string]int // tracks resource names and their depth so that we can stop when the desired depth is reached.
}

func newSimpleGenerator(childDepth, numItems int) *simpleGenerator {
	return &simpleGenerator{
		childDepth: childDepth,
		numItems:   numItems,
		depthMap:   map[string]int{},
	}
}

func (d *simpleGenerator) createResource(apiVersion, kind, name string) *resource.Resource {
	obj := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]any{
			"name": name,
		},
	}
	return &resource.Resource{Unstructured: unstructured.Unstructured{Object: obj}}
}

func (d *simpleGenerator) trackResourceDepth(name string, depth int) {
	d.l.Lock()
	defer d.l.Unlock()
	d.depthMap[name] = depth
}

func (d *simpleGenerator) createRefAtDepth(depth int) v1.ObjectReference {
	prefix := "comp-res"
	if depth == d.childDepth {
		prefix = "managed-res"
	}
	name := fmt.Sprintf("%s-%d-%d", prefix, rand.Int(), depth)
	d.trackResourceDepth(name, depth)
	return v1.ObjectReference{
		Kind:       fmt.Sprintf("Depth%d", depth),
		Name:       name,
		APIVersion: "example.com/v1",
	}
}

func (d *simpleGenerator) createResourceFromRef(ref *v1.ObjectReference) *resource.Resource {
	return d.createResource(ref.APIVersion, ref.Kind, ref.Name)
}

func (d *simpleGenerator) loadResource(_ context.Context, ref *v1.ObjectReference) *resource.Resource {
	return d.createResourceFromRef(ref)
}

func (d *simpleGenerator) depthFromResource(res *resource.Resource) int {
	d.l.Lock()
	defer d.l.Unlock()
	return d.depthMap[res.Unstructured.GetName()]
}

func (d *simpleGenerator) getResourceChildrenRefs(r *resource.Resource) []v1.ObjectReference {
	depth := d.depthFromResource(r)
	if depth == d.childDepth {
		return nil
	}
	ret := make([]v1.ObjectReference, 0, d.numItems)
	for range d.numItems {
		ret = append(ret, d.createRefAtDepth(depth+1))
	}
	return ret
}

var _ resourceLoader = &simpleGenerator{}

func countItems(root *resource.Resource) int {
	ret := 1
	for _, child := range root.Children {
		ret += countItems(child)
	}
	return ret
}

func TestLoader(t *testing.T) {
	type want struct {
		expectedResources int
	}
	type args struct {
		childDepth      int
		numItems        int
		channelCapacity int
		concurrency     int
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Basic": {
			reason: "simple test with default concurrency",
			args: args{
				childDepth: 3,
				numItems:   3,
			},
			want: want{
				expectedResources: 1 + 3 + 9 + 27,
			},
		},
		"BlockingBuffer": {
			reason: "in-process resources greater than channel buffer, causing blocking",
			args: args{
				channelCapacity: 1,
				concurrency:     1,
				childDepth:      3,
				numItems:        10,
			},
			want: want{
				expectedResources: 1 + 10 + 100 + 1000,
			},
		},
		"NoRootChildren": {
			reason: "top-level resource has no children",
			args: args{
				childDepth: 0,
				numItems:   0,
			},
			want: want{
				expectedResources: 1,
			},
		},
		"BadConcurrency": {
			reason: "invalid concurrency is adjusted to be valid",
			args: args{
				concurrency: -1,
				childDepth:  3,
				numItems:    3,
			},
			want: want{
				expectedResources: 1 + 3 + 9 + 27,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			channelCapacity := defaultChannelCapacity
			if test.args.channelCapacity > 0 {
				channelCapacity = test.args.channelCapacity
			}
			concurrency := defaultConcurrency
			if test.args.concurrency != 0 {
				concurrency = test.args.concurrency
			}
			sg := newSimpleGenerator(test.args.childDepth, test.args.numItems)
			rootRef := sg.createRefAtDepth(0)
			root := sg.createResourceFromRef(&rootRef)
			l := newLoader(root, sg, channelCapacity)
			l.load(context.Background(), concurrency)
			n := countItems(root)
			if test.want.expectedResources != n {
				t.Errorf("resource count mismatch: want %d, got %d", test.want.expectedResources, n)
			}
		})
	}
}
