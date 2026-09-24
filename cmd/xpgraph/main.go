/*
Copyright 2026 The Crossplane Authors.

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

// xpgraph prints the ordering graph a composite resource carries.
//
// A composition function that declares composed resource ordering has its
// constraints recorded on the XR, in spec.crossplane.resourceRefs[].dependsOn.
// That is what Crossplane itself orders by - it survives a restart, because
// the replacement process reads it rather than recomputing it - so it is also
// the honest thing to look at when asking why a resource hasn't been created,
// or why a delete is waiting.
//
// The graph alone doesn't answer those questions, though. A resource is
// created once what it depends on reports Ready, so xpgraph fetches each
// composed resource and prints its state beside the edges.
//
// It also reads status.crossplane.pendingResources, where Crossplane records
// what the graph is holding back and why. That matters most for a resource
// held back from being created: it has no composed resource reference at all,
// so without the status it is missing from the graph entirely - which is the
// opposite of what someone asking "why hasn't this been created?" needs to
// see. Resources held back from deletion appear there too, and deadlocks get
// their own block, because they are the only state waiting will not fix.
//
//	xpgraph xordering/ordered -n default
//	xpgraph servingstack/my-stack -n modelplane-system --dot | dot -Tpng -o graph.png
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type cli struct {
	Resource  string `arg:""                                               help:"The composite resource, as kind/name or resource.group/name."`
	Namespace string `help:"Namespace of a namespaced composite resource." short:"n"`

	Kubeconfig string `help:"Path to a kubeconfig. Defaults to $KUBECONFIG, then ~/.kube/config."`
	Context    string `help:"Kubeconfig context to use. Defaults to the current context."`

	Dot     bool          `help:"Print Graphviz DOT instead of a tree."`
	Color   string        `default:"auto"                               enum:"auto,always,never"                            help:"Colorize the tree."`
	Timeout time.Duration `default:"30s"                                help:"How long to spend talking to the API server."`
}

// A node is one composed resource and the edges leading into it.
type node struct {
	Name       string // The composition resource name, which edges refer to.
	APIVersion string
	Kind       string
	Object     string // metadata.name of the composed resource.
	Namespace  string // Set only when a cluster scoped composite composes a namespaced resource.
	DependsOn  []string

	// Requires is the requirement names this resource waits on - resources
	// the pipeline required rather than composed. They gate creation only,
	// and they are not composed resources, so they are not nodes.
	Requires []string

	// Live state, read from the composed resource itself.
	Exists   bool
	Ready    bool
	Deleting bool
	Reason   string // Why it isn't ready, when it says.

	// Held state, read from the XR's status rather than from the resource.
	// A resource the graph is holding back has no object to look at, and in
	// the create direction no reference either: the XR is the only place it
	// appears at all.
	Held       bool   // The graph won't allow this resource's next change yet.
	Operation  string // What is held: "Create" or "Delete".
	Deadlocked bool   // Waiting cannot resolve it. Someone has to act.

	wave int
}

// The states a node can be in. A resource reaches ready, creating, pending
// and deleting on its own; blocked, held and deadlocked are the graph's
// doing, and are the states this tool exists to explain.
const (
	stateReady      = "ready"
	stateCreating   = "creating"
	statePending    = "pending"
	stateDeleting   = "deleting"
	stateBlocked    = "blocked"
	stateHeld       = "held"
	stateDeadlocked = "deadlocked"
)

// A style paints terminal output, or doesn't. Colour is off when stdout
// isn't a terminal, so piping to a file or a pager stays readable.
type style struct{ color bool }

const (
	dim    = "2"
	green  = "32"
	yellow = "33"
	red    = "31"
	gray   = "90"
	bold   = "1"
)

func (s style) paint(code, text string) string {
	if !s.color || code == "" {
		return text
	}

	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// glyph and color are how a state reads at a glance: the shape carries the
// meaning where colour isn't available, so both say the same thing.
func (n node) glyph() (string, string) {
	switch n.state() {
	case stateReady:
		return "✔", green
	case stateCreating:
		return "◐", yellow
	case stateDeleting:
		return "✖", red
	case stateDeadlocked:
		return "⨯", red
	case stateBlocked, stateHeld:
		return "⊘", yellow
	default:
		return "○", gray
	}
}

// state is what the resource is doing, in one word.
// state is what the resource is doing, in one word.
//
// Deadlock outranks everything: it is the only state that will not resolve
// on its own, so it must not be hidden behind a resource that also happens
// to be deleting. Below that, being held by the graph outranks the
// resource's own state, because "Crossplane won't do this yet" is the answer
// to the question someone is asking, and "pending" or "ready" is not.
func (n node) state() string {
	switch {
	case n.Deadlocked:
		return stateDeadlocked
	case n.Held && n.Operation == "Delete":
		return stateHeld
	case n.Held:
		return stateBlocked
	case n.Deleting:
		return stateDeleting
	case !n.Exists:
		return statePending
	case n.Ready:
		return stateReady
	default:
		return stateCreating
	}
}

func main() {
	c := &cli{}
	k := kong.Parse(c,
		kong.Name("xpgraph"),
		kong.Description("Print the ordering graph a composite resource carries."),
		kong.UsageOnError(),
	)
	k.FatalIfErrorf(c.Run())
}

func (c *cli) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	dyn, mapper, err := c.connect()
	if err != nil {
		return err
	}

	xr, err := c.getComposite(ctx, dyn, mapper)
	if err != nil {
		return err
	}

	nodes, err := readGraph(xr)
	if err != nil {
		return err
	}

	// The edges say what waits for what; the composed resources say where
	// each one has got to. Both are needed to read a graph mid-reconcile.
	observe(ctx, dyn, mapper, xr.GetNamespace(), nodes)

	// Last, because it overrides both: what the XR says the graph is holding
	// back, including resources that have no reference to observe.
	nodes = readPending(xr, nodes)

	out := renderTree(xr, nodes, style{color: c.colorize()})
	if c.Dot {
		out = renderDot(xr, nodes)
	}

	if _, err := os.Stdout.WriteString(out); err != nil {
		return fmt.Errorf("cannot write output: %w", err)
	}

	return nil
}

// colorize reports whether to paint the tree. "auto" means only when stdout
// is a terminal.
func (c *cli) colorize() bool {
	switch c.Color {
	case "always":
		return true
	case "never":
		return false
	default:
		fi, err := os.Stdout.Stat()
		return err == nil && fi.Mode()&os.ModeCharDevice != 0
	}
}

func (c *cli) connect() (dynamic.Interface, meta.RESTMapper, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if c.Kubeconfig != "" {
		rules = &clientcmd.ClientConfigLoadingRules{ExplicitPath: c.Kubeconfig}
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{CurrentContext: c.Context},
	).ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot load kubeconfig: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create client: %w", err)
	}

	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create discovery client: %w", err)
	}

	groups, err := restmapper.GetAPIGroupResources(dc)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot discover API groups: %w", err)
	}

	return dyn, restmapper.NewDiscoveryRESTMapper(groups), nil
}

// getComposite resolves kind/name (or resource.group/name) the way kubectl
// does, and fetches it.
func (c *cli) getComposite(ctx context.Context, dyn dynamic.Interface, mapper meta.RESTMapper) (*unstructured.Unstructured, error) {
	typ, name, ok := strings.Cut(c.Resource, "/")
	if !ok {
		return nil, fmt.Errorf("%q is not kind/name", c.Resource)
	}

	var gvk schema.GroupVersionKind

	if res, group, qualified := strings.Cut(typ, "."); qualified {
		m, err := mapper.RESTMapping(schema.GroupKind{Group: group, Kind: res})
		if err != nil {
			// Not a kind.group; try it as a resource.group instead.
			gvr, gerr := mapper.ResourceFor(schema.GroupVersionResource{Group: group, Resource: res})
			if gerr != nil {
				return nil, fmt.Errorf("cannot resolve %q: %w", typ, err)
			}

			gvk, err = mapper.KindFor(gvr)
			if err != nil {
				return nil, fmt.Errorf("cannot resolve %q: %w", typ, err)
			}
		} else {
			gvk = m.GroupVersionKind
		}
	} else {
		var err error
		if gvk, err = mapper.KindFor(schema.GroupVersionResource{Resource: typ}); err != nil {
			return nil, fmt.Errorf("cannot resolve %q: %w", typ, err)
		}
	}

	m, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("cannot map %s: %w", gvk, err)
	}

	xr, err := dyn.Resource(m.Resource).Namespace(c.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot get %s/%s: %w", gvk.Kind, name, err)
	}

	return xr, nil
}

// readGraph reads the composed resource references, and the ordering
// constraints they carry, off the XR.
func readGraph(xr *unstructured.Unstructured) ([]*node, error) {
	refs, found, err := unstructured.NestedSlice(xr.Object, "spec", "crossplane", "resourceRefs")
	if err != nil {
		return nil, fmt.Errorf("cannot read spec.crossplane.resourceRefs: %w", err)
	}

	if !found {
		// Legacy (v1) XRs keep their references at the top of spec.
		if refs, found, err = unstructured.NestedSlice(xr.Object, "spec", "resourceRefs"); err != nil {
			return nil, fmt.Errorf("cannot read spec.resourceRefs: %w", err)
		}
	}

	if !found {
		return nil, fmt.Errorf("%s/%s has no composed resource references", xr.GetKind(), xr.GetName())
	}

	nodes := make([]*node, 0, len(refs))

	for _, r := range refs {
		ref, ok := r.(map[string]any)
		if !ok {
			continue
		}

		n := &node{}
		n.Name, _, _ = unstructured.NestedString(ref, "resourceName")
		n.APIVersion, _, _ = unstructured.NestedString(ref, "apiVersion")
		n.Kind, _, _ = unstructured.NestedString(ref, "kind")
		n.Object, _, _ = unstructured.NestedString(ref, "name")
		n.Namespace, _, _ = unstructured.NestedString(ref, "namespace")

		if n.Name == "" {
			// A reference without a composition resource name predates the
			// annotation, and no edge can name it.
			n.Name = n.Object
		}

		n.DependsOn, n.Requires = readDependsOn(ref)

		nodes = append(nodes, n)
	}

	return nodes, nil
}

// readDependsOn reads an entry's edges, in either form they take.
//
// dependsOn started as a list of composition resource names and is becoming a
// list of objects, so that an edge can say what kind of thing it points at -
// another composed resource, or a resource the pipeline required rather than
// composed. Reading both means one xpgraph works against a cluster on either
// side of that change, which matters because the whole point of this tool is
// looking at a cluster you did not build.
func readDependsOn(ref map[string]any) (composed, requires []string) {
	raw, found, err := unstructured.NestedSlice(ref, "dependsOn")
	if err != nil || !found {
		return nil, nil
	}

	for _, d := range raw {
		switch v := d.(type) {
		case string:
			composed = append(composed, v)
		case map[string]any:
			t, _, _ := unstructured.NestedString(v, "type")
			if t == "RequiredResource" {
				name, _, _ := unstructured.NestedString(v, "requirement", "name")
				if name != "" {
					requires = append(requires, name)
				}

				continue
			}

			if name, _, _ := unstructured.NestedString(v, "name"); name != "" {
				composed = append(composed, name)
			}
		}
	}

	return composed, requires
}

// readPending folds the XR's status.crossplane.pendingResources into the
// graph.
//
// A resource the graph is holding back from being created has no composed
// resource reference - Crossplane deliberately doesn't write one, because a
// reference to an object that doesn't exist reads as an error rather than as
// waiting - so without this it is missing from the graph entirely, which is
// the opposite of what someone asking "why hasn't this been created?" needs
// to see. A resource held back from deletion does have a reference, and this
// says what is holding it.
func readPending(xr *unstructured.Unstructured, nodes []*node) []*node {
	pending, found, err := unstructured.NestedSlice(xr.Object, "status", "crossplane", "pendingResources")
	if err != nil {
		return nodes
	}

	if !found {
		// Legacy (v1) XRs keep their machinery at the top of status.
		if pending, found, err = unstructured.NestedSlice(xr.Object, "status", "pendingResources"); err != nil || !found {
			return nodes
		}
	}

	byName := make(map[string]*node, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	for _, p := range pending {
		e, ok := p.(map[string]any)
		if !ok {
			continue
		}

		name, _, _ := unstructured.NestedString(e, "resourceName")
		if name == "" {
			continue
		}

		n, ok := byName[name]
		if !ok {
			// Held back from creation: nothing references it, so this entry
			// is all there is.
			n = &node{Name: name}
			n.APIVersion, _, _ = unstructured.NestedString(e, "apiVersion")
			n.Kind, _, _ = unstructured.NestedString(e, "kind")
			n.DependsOn, n.Requires = readDependsOn(e)
			nodes = append(nodes, n)
			byName[name] = n
		}

		n.Held = true
		n.Operation, _, _ = unstructured.NestedString(e, "operation")
		n.Deadlocked, _, _ = unstructured.NestedBool(e, "deadlocked")

		if r, _, _ := unstructured.NestedString(e, "reason"); r != "" {
			n.Reason = r
		}
	}

	return nodes
}

// observe fills in each node's live state. A resource that can't be fetched
// is reported as pending rather than failing the run: mid-teardown, half the
// graph is legitimately gone.
func observe(ctx context.Context, dyn dynamic.Interface, mapper meta.RESTMapper, ns string, nodes []*node) {
	for _, n := range nodes {
		if n.Object == "" || n.Kind == "" {
			continue
		}

		gvk, err := kindOf(mapper, n)
		if err != nil {
			continue
		}

		m, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			continue
		}

		ri := dyn.Resource(m.Resource)

		var got *unstructured.Unstructured
		if m.Scope.Name() == meta.RESTScopeNameNamespace {
			// A namespaced composite composes into its own namespace and
			// leaves the reference's namespace empty. A cluster scoped one
			// records the namespace per resource, because they can differ.
			in := n.Namespace
			if in == "" {
				in = ns
			}

			got, err = ri.Namespace(in).Get(ctx, n.Object, metav1.GetOptions{})
		} else {
			got, err = ri.Get(ctx, n.Object, metav1.GetOptions{})
		}

		if err != nil {
			continue
		}

		n.Exists = true
		n.Deleting = got.GetDeletionTimestamp() != nil
		n.Ready = ready(got)

		if !n.Ready {
			n.Reason = notReadyReason(got)
		}
	}
}

func kindOf(mapper meta.RESTMapper, n *node) (schema.GroupVersionKind, error) {
	// The reference carries apiVersion and kind, so the GVK comes straight
	// off it. Older references may carry only a kind, which needs the mapper.
	if n.APIVersion != "" {
		gv, err := schema.ParseGroupVersion(n.APIVersion)
		if err != nil {
			return schema.GroupVersionKind{}, err
		}

		return gv.WithKind(n.Kind), nil
	}

	return mapper.KindFor(schema.GroupVersionResource{Resource: strings.ToLower(n.Kind) + "s"})
}

// ready reports whether a resource looks ready.
//
// Crossplane's own view of readiness comes from the function's response, which
// isn't persisted anywhere xpgraph can read, so this is a best-effort read of
// the resource itself. Ready is the usual condition, but plenty of composed
// resources never set it: a CRD reports Established, a GatewayClass or Gateway
// reports Accepted. Treating those as ready matches what the function almost
// certainly told Crossplane, and without them a healthy graph reads as stuck.
func ready(u *unstructured.Unstructured) bool {
	conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conds {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}

		t, _, _ := unstructured.NestedString(cond, "type")
		s, _, _ := unstructured.NestedString(cond, "status")

		if s != "True" {
			continue
		}

		switch t {
		case "Ready", "Established", "Accepted":
			return true
		}
	}

	// A resource with no conditions at all - a Namespace, a ConfigMap - is
	// ready by existing. Nothing else will ever mark it so.
	return len(conds) == 0
}

// notReadyReason is the reason from the first condition that isn't True, so
// a resource that is stuck says why on its own line rather than needing a
// separate kubectl describe.
func notReadyReason(u *unstructured.Unstructured) string {
	conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conds {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}

		if s, _, _ := unstructured.NestedString(cond, "status"); s == "True" {
			continue
		}

		t, _, _ := unstructured.NestedString(cond, "type")
		r, _, _ := unstructured.NestedString(cond, "reason")

		switch {
		case r != "" && t != "":
			return t + "=" + r
		case r != "":
			return r
		}
	}

	return ""
}

// waves assigns each node a depth: how many levels of dependency sit beneath
// it. Nodes in the same wave can be created concurrently, which is how
// Crossplane treats them. It returns the names of any nodes in a cycle, which
// have no wave.
func waves(nodes []*node) []string {
	byName := map[string]*node{}
	for _, n := range nodes {
		byName[n.Name] = n
		n.wave = -1
	}

	// Repeatedly place every node whose dependencies are all placed. What's
	// left after a pass that places nothing is a cycle.
	for {
		progress := false

		for _, n := range nodes {
			if n.wave >= 0 {
				continue
			}

			w, ok := 0, true

			for _, d := range n.DependsOn {
				dep, exists := byName[d]
				if !exists {
					// An edge to something the XR doesn't reference. Treat it
					// as satisfied rather than stalling the whole graph.
					continue
				}

				if dep.wave < 0 {
					ok = false
					break
				}

				if dep.wave+1 > w {
					w = dep.wave + 1
				}
			}

			if ok {
				n.wave, progress = w, true
			}
		}

		if !progress {
			break
		}
	}

	cyclic := []string{}

	for _, n := range nodes {
		if n.wave < 0 {
			cyclic = append(cyclic, n.Name)
		}
	}

	sort.Strings(cyclic)

	return cyclic
}

func renderTree(xr *unstructured.Unstructured, nodes []*node, st style) string {
	w := &strings.Builder{}

	cyclic := waves(nodes)
	byWave, deepest, counts := group(nodes)

	renderHeader(w, xr, nodes, len(byWave), st)

	nameCol, kindCol := columns(nodes)
	for i := 0; i <= deepest; i++ {
		renderWave(w, i, byWave[i], nameCol, kindCol, st)
	}

	renderTally(w, counts, st)

	if len(cyclic) > 0 {
		fmt.Fprintf(w, "\n%s\n  %s\n", st.paint(red, "cycle, so these can never be created or deleted in order:"),
			strings.Join(cyclic, ", "))
	}

	renderDeadlocked(w, nodes, st)

	return w.String()
}

// renderDeadlocked calls out the resources waiting cannot help.
//
// Everything else in a graph resolves itself if you leave it alone, so a
// long list of waiting resources is not a problem and should not read like
// one. A deadlock is the opposite: it is rare, it is permanent, and someone
// has to do something. It gets its own block at the bottom, with the reason,
// rather than a row in the middle of a wave that looks like all the others.
func renderDeadlocked(w *strings.Builder, nodes []*node, st style) {
	stuck := make([]*node, 0)

	for _, n := range nodes {
		if n.Deadlocked {
			stuck = append(stuck, n)
		}
	}

	if len(stuck) == 0 {
		return
	}

	sort.Slice(stuck, func(a, b int) bool { return stuck[a].Name < stuck[b].Name })

	fmt.Fprintf(w, "\n%s\n", st.paint(red, "deadlocked, so waiting will not resolve these:"))

	for _, n := range stuck {
		reason := n.Reason
		if reason == "" {
			reason = "no reason given"
		}

		fmt.Fprintf(w, "  %s  %s\n", n.Name, st.paint(dim, reason))
	}
}

// group buckets nodes by wave, and counts what each is doing.
func group(nodes []*node) (byWave map[int][]*node, deepest int, counts map[string]int) {
	byWave, counts = map[int][]*node{}, map[string]int{}

	for _, n := range nodes {
		counts[n.state()]++

		if n.wave < 0 {
			continue
		}

		byWave[n.wave] = append(byWave[n.wave], n)

		if n.wave > deepest {
			deepest = n.wave
		}
	}

	return byWave, deepest, counts
}

// columns sizes the name and kind columns to the content, so they line up
// down the whole graph rather than within a wave.
func columns(nodes []*node) (name, kind int) {
	for _, n := range nodes {
		if len(n.Name) > name {
			name = len(n.Name)
		}

		if len(n.Kind) > kind {
			kind = len(n.Kind)
		}
	}

	return name, kind
}

func renderHeader(w *strings.Builder, xr *unstructured.Unstructured, nodes []*node, waves int, st style) {
	where := xr.GetName()
	if xr.GetNamespace() != "" {
		where = xr.GetNamespace() + "/" + where
	}

	edges := 0
	for _, n := range nodes {
		edges += len(n.DependsOn)
	}

	fmt.Fprintf(w, "%s %s", st.paint(bold, xr.GetKind()), where)

	if r := xrReady(xr); r != "" {
		fmt.Fprintf(w, "   %s", st.paint(dim, "Ready="+r))
	}

	fmt.Fprintf(w, "\n%s\n\n", st.paint(dim, fmt.Sprintf("%d composed resources · %d edges · %d waves",
		len(nodes), edges, waves)))

	if edges == 0 {
		fmt.Fprintf(w, "%s\n", st.paint(yellow, "No ordering constraints."))
		fmt.Fprintf(w, "%s\n\n", st.paint(dim,
			"Either the pipeline declares none, or this Crossplane doesn't support\nthem (--enable-composed-resource-ordering)."))
	}
}

func renderWave(w *strings.Builder, i int, ns []*node, nameCol, kindCol int, st style) {
	if len(ns) == 0 {
		return
	}

	sort.Slice(ns, func(a, b int) bool { return ns[a].Name < ns[b].Name })

	label := fmt.Sprintf("wave %d ", i)
	fmt.Fprintf(w, "%s\n", st.paint(dim, label+strings.Repeat("─", maxInt(0, 66-len(label)))))

	for _, n := range ns {
		renderNode(w, n, nameCol, kindCol, st)
	}

	fmt.Fprintln(w)
}

func renderNode(w *strings.Builder, n *node, nameCol, kindCol int, st style) {
	g, c := n.glyph()

	// Glyph, state word, name, kind - then why, if it isn't ready.
	// 10, the width of "deadlocked", so the longest state still lines up.
	row := fmt.Sprintf(" %s %s  %s  %s", st.paint(c, g), st.paint(c, padRight(n.state(), 10)),
		padRight(n.Name, nameCol), st.paint(dim, padRight(n.Kind, kindCol)))

	if n.Reason != "" {
		row += "  " + st.paint(yellow, n.Reason)
	}

	fmt.Fprintln(w, strings.TrimRight(row, " "))

	// Dependencies go on their own indented lines, wrapped. A resource
	// waiting on a whole CRD bundle has a dozen of them, and one long line
	// buries the rest of the graph.
	if len(n.DependsOn) > 0 {
		deps := append([]string{}, n.DependsOn...)
		sort.Strings(deps)
		renderEdges(w, "←", deps, st)
	}

	// Required resources get their own arrow. They are not composed by this
	// XR and so are not nodes in the graph - they gate creation and nothing
	// else - and showing them as ordinary edges would suggest Crossplane
	// will delete them in order too, which it never does.
	if len(n.Requires) > 0 {
		req := append([]string{}, n.Requires...)
		sort.Strings(req)
		renderEdges(w, "⇠", req, st)
	}
}

func renderEdges(w *strings.Builder, lead string, names []string, st style) {
	for i, line := range wrap(names, 62) {
		l := lead
		if i > 0 {
			l = " "
		}

		fmt.Fprintf(w, "     %s\n", st.paint(dim, l+" "+line))
	}
}

// renderTally is a one-line summary, in the order a graph moves through:
// what's done, what's working, what hasn't started, what's going away.
func renderTally(w *strings.Builder, counts map[string]int, st style) {
	parts := []string{}

	for _, s := range []struct {
		state string
		code  string
	}{
		{stateReady, green},
		{stateCreating, yellow},
		{stateBlocked, yellow},
		{statePending, gray},
		{stateHeld, yellow},
		{stateDeleting, red},
		{stateDeadlocked, red},
	} {
		if counts[s.state] > 0 {
			parts = append(parts, st.paint(s.code, fmt.Sprintf("%d %s", counts[s.state], s.state)))
		}
	}

	if len(parts) > 0 {
		fmt.Fprintf(w, "%s\n", strings.Join(parts, st.paint(dim, " · ")))
	}
}

// xrReady is the composite's own Ready condition, for context above the graph.
func xrReady(xr *unstructured.Unstructured) string {
	conds, _, _ := unstructured.NestedSlice(xr.Object, "status", "conditions")
	for _, c := range conds {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}

		if t, _, _ := unstructured.NestedString(cond, "type"); t == "Ready" {
			s, _, _ := unstructured.NestedString(cond, "status")
			return s
		}
	}

	return ""
}

// padRight pads to width, counting runes rather than bytes so a name with
// non-ASCII characters still lines up.
func padRight(s string, width int) string {
	if n := width - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}

	return s
}

// wrap joins names into lines no longer than width, breaking between names.
func wrap(names []string, width int) []string {
	lines := []string{}
	cur := ""

	for _, n := range names {
		switch {
		case cur == "":
			cur = n
		case len(cur)+2+len(n) <= width:
			cur += ", " + n
		default:
			lines = append(lines, cur+",")
			cur = n
		}
	}

	if cur != "" {
		lines = append(lines, cur)
	}

	return lines
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func renderDot(xr *unstructured.Unstructured, nodes []*node) string {
	w := &strings.Builder{}

	fmt.Fprintf(w, "digraph %q {\n", xr.GetKind()+"/"+xr.GetName())
	fmt.Fprintln(w, "  rankdir=BT;")
	fmt.Fprintln(w, "  node [shape=box style=rounded fontname=\"Helvetica\"];")

	sorted := append([]*node{}, nodes...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].Name < sorted[b].Name })

	for _, n := range sorted {
		// The label is written directly rather than through %q, which would
		// escape the backslash and print a literal \n in the node.
		fmt.Fprintf(w, "  %q [label=\"%s\\n%s\" color=%q%s];\n",
			n.Name, n.Name, n.Kind, dotColor(n), dotStyle(n))
	}

	// Edges point from a resource to what it waits for, which is also the
	// order teardown runs in.
	for _, n := range sorted {
		deps := append([]string{}, n.DependsOn...)
		sort.Strings(deps)

		for _, d := range deps {
			fmt.Fprintf(w, "  %q -> %q;\n", n.Name, d)
		}
	}

	// Required resources are drawn, but as a different kind of thing: they
	// are not composed by this XR, they only gate it. A dashed edge to a
	// note-shaped node says "this is outside the graph" without leaving it
	// off the picture, which would hide why a resource is waiting.
	for _, n := range sorted {
		req := append([]string{}, n.Requires...)
		sort.Strings(req)

		for _, r := range req {
			fmt.Fprintf(w, "  %q [shape=note color=gray60];\n", r)
			fmt.Fprintf(w, "  %q -> %q [style=dashed color=gray60];\n", n.Name, r)
		}
	}

	fmt.Fprintln(w, "}")

	return w.String()
}

// dotStyle ghosts what doesn't exist and thickens what needs attention, so a
// rendered graph reads without a legend.
func dotStyle(n *node) string {
	switch n.state() {
	case stateDeadlocked:
		return " style=\"rounded,filled\" fillcolor=mistyrose penwidth=2"
	case stateBlocked, stateHeld:
		return " style=\"rounded,bold\""
	case statePending:
		return " style=\"rounded,dashed\""
	default:
		return ""
	}
}

func dotColor(n *node) string {
	switch n.state() {
	case stateReady:
		return "green4"
	case stateDeleting, stateDeadlocked:
		return "red3"
	case stateCreating, stateBlocked, stateHeld:
		return "goldenrod3"
	default:
		return "gray60"
	}
}
