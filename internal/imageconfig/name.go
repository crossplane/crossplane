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

// Package imageconfig derives package object names from ImageConfigs.
package imageconfig

import (
	"context"
	"strings"
	"text/template"

	"github.com/google/go-containerregistry/pkg/name"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xpkg"

	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
)

const (
	errListImageConfigs = "cannot list ImageConfigs"
	errParseTemplate    = "cannot parse packageName template"
	errExecuteTemplate  = "cannot render packageName template"

	errFmtEmptyName = "ImageConfig %q produced an empty package name for image %q"
)

const (
	// Package name templates are delimited by << and >> rather than Go's usual
	// {{ and }}, so that an ImageConfig can be shipped in a Helm chart without
	// Helm consuming the template when it renders the chart. A packageName
	// written with Go's default delimiters is rejected by the API server,
	// since it contains no << and isn't a valid DNS label.
	leftDelim  = "<<"
	rightDelim = ">>"
)

// TemplateData is the data a packageName template is rendered with.
type TemplateData struct {
	// Image is the image path, without its tag or digest. For example
	// "xpkg.crossplane.io/crossplane-contrib/provider-aws".
	Image string

	// Registry is the registry part of Image, for example
	// "xpkg.crossplane.io". It is empty if Image doesn't include a registry.
	Registry string

	// Repository is Image without its registry, for example
	// "crossplane-contrib/provider-aws".
	Repository string

	// Path is Repository without its last segment, for example
	// "crossplane-contrib". It is empty if Repository has only one segment.
	Path string

	// Name is the last segment of Repository, for example "provider-aws".
	Name string

	// Match is the matchImages prefix that selected the ImageConfig, for
	// example "xpkg.crossplane.io/crossplane-contrib/".
	Match string

	// Remainder is Image with Match trimmed from it, for example
	// "provider-aws".
	Remainder string

	// ImageConfig is the name of the ImageConfig that matched the image.
	ImageConfig string
}

// templateFuncs returns the functions available to a packageName template. The
// string being operated on comes last so that it reads well at the end of a
// pipeline, e.g. << trimPrefix "provider-" .Name >>.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"lower":      strings.ToLower,
		"trimPrefix": func(prefix, s string) string { return strings.TrimPrefix(s, prefix) },
		"trimSuffix": func(suffix, s string) string { return strings.TrimSuffix(s, suffix) },
		"replace":    func(from, to, s string) string { return strings.ReplaceAll(s, from, to) },
	}
}

// A Namer derives the name of the Crossplane package object (a Provider,
// Configuration, or Function) that should be created for a package image.
type Namer struct {
	client client.Reader
}

// NewNamer creates a Namer that reads ImageConfigs using the supplied client.
func NewNamer(c client.Reader) *Namer {
	return &Namer{client: c}
}

// Name returns the object name to use for the supplied package reference,
// together with the name of the ImageConfig that determined it. When no
// ImageConfig with a packageName matches the image the name is derived from the
// image's repository path, and the returned ImageConfig name is empty.
//
// The reference must be the package's original source, i.e. the source before
// any ImageConfig path rewriting was applied. Package names are derived from
// original sources so that they don't change when a mirror is configured.
func (n *Namer) Name(ctx context.Context, ref name.Reference) (objName, imageConfig string, err error) {
	src := xpkg.ParsePackageSourceFromReference(ref)

	cfg, match, err := n.bestMatch(ctx, src)
	if err != nil {
		return "", "", err
	}

	if cfg == nil {
		return xpkg.ToDNSLabel(ref.Context().RepositoryStr()), "", nil
	}

	objName, err = packageName(cfg, src, match)
	if err != nil {
		return "", cfg.GetName(), err
	}

	return objName, cfg.GetName(), nil
}

// bestMatch returns the ImageConfig with a packageName whose matchImages prefix
// is the longest one matching the supplied image, along with that prefix. It
// returns a nil ImageConfig if none match, which is not an error.
func (n *Namer) bestMatch(ctx context.Context, image string) (*v1beta1.ImageConfig, string, error) {
	l := &v1beta1.ImageConfigList{}
	if err := n.client.List(ctx, l); err != nil {
		return nil, "", errors.Wrap(err, errListImageConfigs)
	}

	var (
		cfg   *v1beta1.ImageConfig
		match string
	)

	for i := range l.Items {
		c := &l.Items[i]
		if c.Spec.PackageName == "" {
			continue
		}

		for _, m := range c.Spec.MatchImages {
			if strings.HasPrefix(image, m.Prefix) && len(m.Prefix) > len(match) {
				cfg = c
				match = m.Prefix
			}
		}
	}

	return cfg, match, nil
}

// packageName renders the supplied ImageConfig's packageName for an image. A
// packageName with no template actions renders to itself, so a literal name
// needs no special handling here.
func packageName(cfg *v1beta1.ImageConfig, image, match string) (string, error) {
	t, err := template.New("packageName").Delims(leftDelim, rightDelim).Funcs(templateFuncs()).Option("missingkey=error").Parse(cfg.Spec.PackageName)
	if err != nil {
		return "", errors.Wrap(err, errParseTemplate)
	}

	b := &strings.Builder{}
	if err := t.Execute(b, templateData(cfg, image, match)); err != nil {
		return "", errors.Wrap(err, errExecuteTemplate)
	}

	// A literal name is validated by the API server, but a template can render
	// to anything, so we always normalize. A name that normalizes to nothing is
	// a configuration error we can't recover from.
	out := xpkg.ToDNSLabel(strings.TrimSpace(b.String()))
	if out == "" {
		return "", errors.Errorf(errFmtEmptyName, cfg.GetName(), image)
	}

	return out, nil
}

func templateData(cfg *v1beta1.ImageConfig, image, match string) TemplateData {
	registry, repository := splitImage(image)

	path, pkgName := "", repository
	if i := strings.LastIndex(repository, "/"); i >= 0 {
		path, pkgName = repository[:i], repository[i+1:]
	}

	return TemplateData{
		Image:       image,
		Registry:    registry,
		Repository:  repository,
		Path:        path,
		Name:        pkgName,
		Match:       match,
		Remainder:   strings.TrimPrefix(image, match),
		ImageConfig: cfg.GetName(),
	}
}

// splitImage splits an image path into its registry and repository parts. We
// split the raw image path rather than using go-containerregistry's parsed
// reference because it normalizes the registry (e.g. docker.io becomes
// index.docker.io), and we want the template to see the path the user wrote.
func splitImage(image string) (registry, repository string) {
	first, rest, ok := strings.Cut(image, "/")
	if !ok {
		return "", image
	}

	if !strings.ContainsAny(first, ".:") && first != "localhost" {
		// The first segment isn't a registry, e.g. "crossplane-contrib/xyz".
		return "", image
	}

	return first, rest
}
