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

// Package workspace contains utilities for working with Crossplane project
// workspaces. Mostly used for storing and validating packages.
package workspace

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"go.lsp.dev/uri"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	xparser "github.com/crossplane/crossplane-runtime/pkg/parser"

	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
	pyaml "github.com/crossplane/crossplane/internal/xpkg/v2/parser/yaml"
	"github.com/crossplane/crossplane/internal/xpkg/v2/workspace/meta"
)

// paths to extract GVK and name from objects that conform to Kubernetes
// standard.
var (
	compResources *yaml.Path
	compBase      *yaml.Path
)

const (
	yamlExt = ".yaml"

	errCompositionResources = "resources in Composition are malformed"
	errInvalidFileURI       = "invalid path supplied"
	errInvalidPackage       = "invalid package; more than one meta (configuration or provider) file supplied"
)

// builds static YAML path strings ahead of usage.
func init() {
	var err error
	compResources, err = yaml.PathString("$.spec.resources")
	if err != nil {
		panic(err)
	}
	compBase, err = yaml.PathString("$.base")
	if err != nil {
		panic(err)
	}
}

// Workspace provides APIs for interacting with the current project workspace.
type Workspace struct {
	// fs represents the filesystem the workspace resides in.
	fs afero.Fs

	log logging.Logger

	mu sync.RWMutex
	// root represents the "root" of the workspace filesystem.
	root string
	view *View
}

// New creates a new Workspace instance.
func New(root string, opts ...Option) (*Workspace, error) {
	w := &Workspace{
		fs:   afero.NewOsFs(),
		log:  logging.NewNopLogger(),
		root: root,
		view: &View{
			examples: make(map[schema.GroupVersionKind][]Node),
			// Default metaLocation to the root. If a pre-existing meta file exists,
			// metaLocation will be updating accordingly during workspace parse.
			metaLocation: root,
			nodes:        make(map[NodeIdentifier]Node),
			uriToDetails: make(map[uri.URI]*Details),
			xrClaimRefs:  make(map[schema.GroupVersionKind]schema.GroupVersionKind),

			root: root,

			printer: &pterm.BasicTextPrinter{Writer: io.Discard},
		},
	}

	p, err := pyaml.New()
	if err != nil {
		return nil, err
	}

	w.view.parser = p

	// apply overrides if they exist
	for _, o := range opts {
		o(w)
	}

	return w, nil
}

// Option represents an option that can be applied to Workspace.
type Option func(*Workspace)

// WithFS overrides the Workspace's filesystem with the supplied filesystem.
func WithFS(fs afero.Fs) Option {
	return func(w *Workspace) {
		w.fs = fs
	}
}

// WithLogger overrides the default logger for the Workspace with the supplied
// logger.
func WithLogger(l logging.Logger) Option {
	return func(w *Workspace) {
		w.log = l
	}
}

// WithPrinter overrides the printer of the Workspace with the supplied
// printer. By default a Workspace has no printer.
func WithPrinter(p pterm.TextPrinter) Option {
	return func(w *Workspace) {
		w.view.printer = p
	}
}

// WithPermissiveParser lets the workspace parser just print warnings when
// a file or a document in a file cannot be parsed. This can be used when
// partial results are more important than correctness.
func WithPermissiveParser() Option {
	return func(w *Workspace) {
		w.view.permissiveParser = true
	}
}

// Write writes the supplied Meta details to the fs.
func (w *Workspace) Write(m *meta.Meta) error {
	b, err := m.Bytes()
	if err != nil {
		return err
	}

	return afero.WriteFile(w.fs, filepath.Join(w.view.metaLocation, xpkg.MetaFile), b, os.ModePerm)
}

// Parse parses the full workspace in order to hydrate the workspace's View.
func (w *Workspace) Parse(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var errs []error
	if err := afero.Walk(w.fs, w.root, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(p) != yamlExt {
			return nil
		}
		// We attempt to parse subsequent documents if we encounter an error
		// in a preceding one.
		// TODO(hasheddan): errors should be aggregated and returned as
		// diagnostics.

		b, err := afero.ReadFile(w.fs, p)
		if err != nil {
			return err
		}

		// add file contents to our inmem workspace
		w.view.uriToDetails[uri.New(p)] = &Details{
			Body:    b,
			NodeIDs: make(map[NodeIdentifier]struct{}),
		}

		if err := w.view.ParseFile(ctx, p); err != nil {
			errs = append(errs, err)
		}
		return nil
	}); err != nil {
		return err
	}

	return kerrors.NewAggregate(errs)
}

// View returns the Workspace's View. Note: this will only exist _after_
// the Workspace has been parsed.
func (w *Workspace) View() *View {
	return w.view
}

// ParseFile parses all YAML objects at the given path and updates the workspace
// node cache.
func (v *View) ParseFile(ctx context.Context, path string) error {
	details, ok := v.uriToDetails[uri.New(path)]
	if !ok {
		return errors.New(errInvalidFileURI)
	}

	f, err := parser.ParseBytes(details.Body, parser.ParseComments)
	if err != nil {
		if v.permissiveParser {
			v.printer.Printfln("WARNING: ignoring file %s: %v", path, err)
			return nil
		}
		return errors.Wrapf(err, "failed to parse file %s", v.relativePath(path))
	}

	var errs []error
	for i, doc := range f.Docs {
		if doc.Body != nil {
			pCtx := parseContext{
				node:     doc,
				path:     path,
				rootNode: true,
			}
			if _, err := v.parseDoc(ctx, pCtx); err != nil {
				if v.permissiveParser {
					if len(f.Docs) > 1 {
						v.printer.Printfln("WARNING: ignoring document %d in file %s: %v", i+1, path, err)
					} else {
						v.printer.Printfln("WARNING: ignoring file %s: %v", path, err)
					}
					continue
				}

				// We attempt to parse subsequent documents if we encounter an error
				// in a preceding one.
				errs = append(errs, err)
				continue
			}
		}
	}

	if len(errs) > 0 {
		return errors.Wrapf(kerrors.NewAggregate(errs), "failed to parse file %s", path)
	}

	return nil
}

type parseContext struct {
	docBytes []byte
	node     ast.Node
	obj      unstructured.Unstructured
	path     string
	doc      int
	rootNode bool
}

// parseDoc recursively parses a YAML document into PackageNodes. Embedded nodes
// are added to the parent's list of dependants.
func (v *View) parseDoc(ctx context.Context, pCtx parseContext) (NodeIdentifier, error) { //nolint:gocyclo // TODO(lsviben) this is complex but most of it is a switch statement
	b, err := pCtx.node.MarshalYAML()
	if err != nil {
		return NodeIdentifier{}, err
	}
	pCtx.docBytes = b

	// first try to unmarshal as pure YAML, not expecting this to be a Kubernetes object.
	var value interface{}
	if err := k8syaml.Unmarshal(b, &value); err != nil {
		return NodeIdentifier{}, err
	}

	// then try to unmarshal as a Kubernetes object. Ignore if errors which means it's not a Kubernetes object.
	var obj unstructured.Unstructured
	// TODO(hasheddan): we cannot make use of strict unmarshal to identify
	// extraneous fields because we don't have the underlying Go types. In
	// the future, we would like to provide warnings on fields that are
	// extraneous, but we will likely need to augment the OpenAPI validation
	// to do so.
	if err := k8syaml.Unmarshal(b, &obj); err != nil {
		v.printer.Printfln("WARNING: ignoring document %d in file %s: missing 'kind' field, not a Kubernetes object", pCtx.doc+1, v.relativePath(pCtx.path))
		return NodeIdentifier{}, nil //nolint:nilerr //explained above
	}
	pCtx.obj = obj
	// NOTE(hasheddan): if we are at document root (i.e. this is a
	// DocumentNode), we must set the underlying ast.Node to the document body
	// so that we can access child nodes generically in validation.
	if doc, ok := pCtx.node.(*ast.DocumentNode); ok {
		pCtx.node = doc.Body
	}

	switch obj.GetKind() {
	case xpextv1.CompositeResourceDefinitionKind:
		if err := v.parseXRD(pCtx); err != nil {
			return NodeIdentifier{}, err
		}
	case xpextv1.CompositionKind:
		if err := v.parseComposition(ctx, pCtx); err != nil {
			return NodeIdentifier{}, err
		}
	case pkgmetav1.ConfigurationKind:
		if err := v.parseMeta(ctx, pCtx); err != nil {
			return NodeIdentifier{}, err
		}
	case pkgmetav1.ProviderKind:
		if err := v.parseMeta(ctx, pCtx); err != nil {
			return NodeIdentifier{}, err
		}
	default:
		v.parseExample(pCtx)
	}
	// TODO(hasheddan): if this is an embedded resource we don't have a name so
	// we should form a deterministic name based on its parent Composition.
	id := nodeID(obj.GetName(), obj.GroupVersionKind())

	v.nodes[id] = &PackageNode{
		ast:      pCtx.node,
		fileName: pCtx.path,
		gvk:      obj.GroupVersionKind(),
		obj:      &obj,
	}

	if pCtx.rootNode {
		v.appendID(pCtx.path, id)
	}

	return id, nil
}

func (v *View) parseComposition(ctx context.Context, pCtx parseContext) error {
	var cp xpextv1.Composition
	if err := k8syaml.Unmarshal(pCtx.docBytes, &cp); err != nil {
		// we have a composition but failed to unmarshal it, skip for now.
		return nil //nolint:nilerr //explained above
	}

	resNode, err := compResources.FilterNode(pCtx.node)
	if err != nil {
		return err
	}
	seq, ok := resNode.(*ast.SequenceNode)
	if !ok {
		// NOTE(hasheddan): if the Composition's resources field is not a
		// sequence node, we skip parsing embedded resources because the
		// Composition itself is malformed.
		return errors.New(errCompositionResources)
	}

	dependants := map[NodeIdentifier]struct{}{}

	for _, s := range seq.Values {
		// process ComposedTemplate
		b, err := s.MarshalYAML()
		if err != nil {
			return err
		}

		var ct xpextv1.ComposedTemplate
		if err := k8syaml.Unmarshal(b, &ct); err != nil {
			return err
		}

		// recurse into resource[i].base
		sNode, err := compBase.FilterNode(s)
		if err != nil {
			// TODO(hasheddan): surface this error as a diagnostic.
			continue
		}
		pCtx.node = sNode
		pCtx.rootNode = false

		id, err := v.parseDoc(ctx, pCtx)
		if err != nil {
			// TODO(hasheddan): surface this error as a diagnostic.
			continue
		}
		dependants[id] = struct{}{}
	}
	return nil
}

func (v *View) parseExample(ctx parseContext) {
	// NOTE(@tnthornton): we handle example claims specially so that we have
	// them available for CompositeTemplate validation.
	if strings.Contains(filepath.Dir(ctx.path), "example") {
		curr, ok := v.examples[ctx.obj.GroupVersionKind()]
		if !ok {
			curr = make([]Node, 0)
		}

		curr = append(curr, &PackageNode{
			ast:      ctx.node,
			fileName: ctx.path,
			gvk:      ctx.obj.GroupVersionKind(),
			obj:      &ctx.obj,
		})

		v.examples[ctx.obj.GroupVersionKind()] = curr
	}
}

func (v *View) parseMeta(ctx context.Context, pCtx parseContext) error {
	v.metaLocation = filepath.Dir(pCtx.path)
	p, err := v.parser.Parse(ctx, io.NopCloser(bytes.NewReader(pCtx.docBytes)))
	if err != nil {
		return err
	}

	if len(p.GetMeta()) != 1 {
		return errors.Errorf("%s in %s", errInvalidPackage, v.relativePath(pCtx.path))
	}

	if v.meta != nil {
		return errors.Errorf("%s: %s, %s and maybe more", errInvalidPackage, v.relativePath(v.metaPath), v.relativePath(pCtx.path))
	}

	v.meta = meta.New(p.GetMeta()[0])
	v.metaPath = pCtx.path

	v.printer.Printf("xpkg loaded package meta information from %s\n", v.relativePath(pCtx.path))

	return nil
}

func (v *View) parseXRD(ctx parseContext) error {
	var xrd xpextv1.CompositeResourceDefinition
	if err := k8syaml.Unmarshal(ctx.docBytes, &xrd); err != nil {
		return err
	}

	v.xrClaimRefs[xrd.GetCompositeGroupVersionKind()] = xrd.GetClaimGroupVersionKind()
	return nil
}

func (v *View) appendID(path string, id NodeIdentifier) {
	uri := uri.New(path)
	curr, ok := v.uriToDetails[uri]
	if !ok {
		v.uriToDetails[uri] = &Details{
			NodeIDs: map[NodeIdentifier]struct{}{
				id: {},
			},
		}
		return
	}
	curr.NodeIDs[id] = struct{}{}

	v.uriToDetails[uri] = curr
}

// nodeID constructs a NodeIdentifier from name and GVK.
func nodeID(name string, gvk schema.GroupVersionKind) NodeIdentifier {
	return NodeIdentifier{
		name: name,
		gvk:  gvk,
	}
}

// View represents the current processed view of the workspace.
type View struct {
	// examples holds a quick access map of GVK -> []Nodes representing the
	// example/**/*.yaml claims for the package.
	examples map[schema.GroupVersionKind][]Node
	// parser is the parser used for parsing packages.
	parser *xparser.PackageParser
	// metaLocation denotes the place the meta file exists at the time the
	// workspace was created.
	metaLocation string
	meta         *meta.Meta
	metaPath     string
	uriToDetails map[uri.URI]*Details
	nodes        map[NodeIdentifier]Node
	// xrClaimRefs defines an look up from XR GVK -> Claim GVK (if one is defined).
	xrClaimRefs map[schema.GroupVersionKind]schema.GroupVersionKind
	// root is the path of the workspace root.
	root    string
	printer pterm.TextPrinter
	// permissiveParser indicates whether to skip files or documents with parse errors.
	permissiveParser bool
}

// FileDetails returns the map of file details found within the parsed workspace.
func (v *View) FileDetails() map[uri.URI]*Details {
	return v.uriToDetails
}

// Meta returns the View's Meta.
func (v *View) Meta() *meta.Meta {
	return v.meta
}

// MetaLocation returns the meta file's location (on disk) in the current View.
func (v *View) MetaLocation() string {
	return v.metaLocation
}

// Nodes returns the View's Nodes.
func (v *View) Nodes() map[NodeIdentifier]Node {
	return v.nodes
}

// Examples returns the View's Nodes corresponding to the files found under
// /examples.
func (v *View) Examples() map[schema.GroupVersionKind][]Node {
	return v.examples
}

// XRClaimsRefs returns a map of XR GVK -> XRC GVK.
func (v *View) XRClaimsRefs() map[schema.GroupVersionKind]schema.GroupVersionKind {
	return v.xrClaimRefs
}

func (v *View) relativePath(path string) string {
	if !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(v.root, path)
	if err != nil {
		return path
	}
	return rel
}

// Details represent file specific details.
type Details struct {
	Body    []byte
	NodeIDs map[NodeIdentifier]struct{}
}

// A Node is a single object in the package workspace graph.
type Node interface {
	GetAST() ast.Node
	GetFileName() string
	GetDependants() []NodeIdentifier
	GetGVK() schema.GroupVersionKind
	GetObject() runtime.Object
}

// NodeIdentifier is the unique identifier of a node in a workspace.
type NodeIdentifier struct {
	name string
	gvk  schema.GroupVersionKind
}

// A PackageNode represents a concrete node in an xpkg.
// TODO(hasheddan): PackageNode should be refactored into separate
// implementations for each node type (e.g. XRD, Composition, CRD, etc.).
type PackageNode struct {
	ast      ast.Node
	fileName string
	gvk      schema.GroupVersionKind
	obj      runtime.Object
}

// GetAST gets the YAML AST node for this package node.
func (p *PackageNode) GetAST() ast.Node {
	return p.ast
}

// GetFileName gets the name of the file for this node.
func (p *PackageNode) GetFileName() string {
	return p.fileName
}

// GetDependants gets the set of nodes dependant on this node.
// TODO(hasheddan): this method signature may change depending on how we want to
// construct the node graph for a workspace.
func (p *PackageNode) GetDependants() []NodeIdentifier {
	return nil
}

// GetGVK returns the GroupVersionKind of this node.
func (p *PackageNode) GetGVK() schema.GroupVersionKind {
	return p.gvk
}

// GetObject returns the runtime.Object for this node.
func (p *PackageNode) GetObject() runtime.Object {
	return p.obj
}
