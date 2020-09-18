/*
Copyright 2020 The Crossplane Authors.

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

package pkg

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const (
	// DockerTarRoot is the path that files copied into the docker "context" will sit under
	// By creating a single virtual directory in the tar we are able to issue a single ADD directive copying to '.'/CWD
	// allowing the package streamer to effectively cat the contents of WORKDIR
	DockerTarRoot = "crossplane-package"
)

// DefaultImagePullOptions is a blank set of docker image pull options
// TODO: look into whether we need to customize these, possibly need to set a cache bust option
var DefaultImagePullOptions = types.ImagePullOptions{}

// BuildCmd builds a package.
type BuildCmd struct {
	Name string `arg:"" optional:"" name:"name" help:"Name of the package to be built. Defaults to name in crossplane.yaml."`

	Tag         string   `short:"t" help:"Package version tag." default:"latest"`
	Registry    string   `short:"r" help:"Package registry." default:"registry.upbound.io"`
	BaseImage   string   `short:"i" help:"image name to build FROM" default:"registry.upbound.io/crossplane/pkg-unpack:latest"`
	PackageRoot string   `short:"f" help:"Path to crossplane.yaml" default:"."`
	Ignore      []string `name:"ignore" help:"Paths, specified relative to --package-root, to exclude from the package."`
	NoPull      bool     `help:"Flag to disable pulling the base image."`
	NoLint      bool     `help:"Flag to disable linting the package image after building."`
}

// Build runs the Build command.
func (b *BuildCmd) Build(ctx context.Context, docker *client.Client) error {
	root, err := filepath.Abs(b.PackageRoot)
	if err != nil {
		return err
	}
	if b.Name == "" {
		yamlPath := filepath.Join(root, "crossplane.yaml")
		b.Name, err = parseNameFromPackageFile(yamlPath)
		if err != nil {
			return err
		}
	}

	bc := &buildImage{
		baseImage:    b.BaseImage,
		pkgImageName: b.FullImageName(),
		logWriter:    os.Stdout,
		pullOpts:     DefaultImagePullOptions,
		basePath:     root,
		docker:       docker,
		noPull:       b.NoPull,
	}
	bc.InclusionFilter = buildDefaultInclusionFilter(root, DockerTarRoot)
	if len(b.Ignore) > 0 {
		bc.InclusionFilter = buildIgnoreFilter(root, b.Ignore, bc.InclusionFilter)
	}

	err = bc.Build(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\nImage tagged as '%s' (package '%s' built from context in '%s')\n", b.FullImageName(), b.Name, root)

	return nil
}

// FullImageName is the docker image name that will be used in all instances
// it is exported so that it can be used by the configuration/provider subcommands
// which embed BuildCmd
func (b *BuildCmd) FullImageName() string {
	return fmt.Sprintf("%s/%s:%s", b.Registry, b.Name, b.Tag)
}

type buildImage struct {
	docker          *client.Client
	baseImage       string
	pkgImageName    string
	logWriter       io.Writer
	pullOpts        types.ImagePullOptions
	basePath        string
	InclusionFilter InclusionFilter
	noPull          bool
}

func (b *buildImage) Build(ctx context.Context) error {
	err := b.pullBase(ctx)
	if err != nil {
		return err
	}

	dockerfile, err := b.templateDockerfile()
	if err != nil {
		return err
	}

	return b.dockerBuild(ctx, dockerfile)
}

func (b *buildImage) pullBase(ctx context.Context) error {
	if b.noPull {
		fmt.Println("Skipping image pull based on --no-pull=true")
		return nil
	}
	dra, err := NewDockerRegistryAuth(ctx, b.baseImage)
	if err != nil {
		return err
	}
	options, err := dra.BuildPullOpts(ctx)
	if err != nil {
		return err
	}
	resp, err := b.docker.ImagePull(ctx, dra.NormalizedImageName(), options)
	if err != nil {
		return err
	}

	_, err = io.Copy(b.logWriter, resp)
	if err != nil {
		return err
	}
	return resp.Close()
}

func (b *buildImage) templateDockerfile() (string, error) {
	tmpl := `
FROM {{.BaseImage}}

# We assume that WORKDIR has been set by the base image
# to control where files will be placed by docker when we specify '.'/CWD
ADD {{.PackageContentsPath}} .
`
	t := template.Must(template.New("dockerfile").Parse(tmpl))
	buf := new(bytes.Buffer)
	values := struct {
		BaseImage           string
		PackageContentsPath string
	}{
		BaseImage:           b.baseImage,
		PackageContentsPath: DockerTarRoot,
	}
	err := t.Execute(buf, values)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// dockerBuild invokes docker's /build api
// https://docs.docker.com/engine/api/v1.40/#operation/ImageBuild
func (b *buildImage) dockerBuild(ctx context.Context, dockerfile string) error {
	cops, err := b.copyOperations()
	if err != nil {
		return err
	}

	tb, err := b.buildTar(dockerfile, cops)
	if err != nil {
		return err
	}

	// not setting NoCache or PullParent to make local development workflows easier
	// TODO: do we need to expose these as flags or just leave them off (default)
	opts := types.ImageBuildOptions{
		Tags: []string{b.pkgImageName},
	}
	resp, err := b.docker.ImageBuild(ctx, tb, opts)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	fmt.Println(buf.String())

	return nil
}

func (b *buildImage) buildTar(dockerfile string, cops []copyOperation) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	// Start by writing the Dockerfile in at the root
	dhdr := &tar.Header{
		Name: "Dockerfile",
		Mode: 0600,
		Size: int64(len(dockerfile)),
	}
	if err := tw.WriteHeader(dhdr); err != nil {
		return nil, err
	}
	_, err := io.Copy(tw, bytes.NewBufferString(dockerfile))
	if err != nil {
		return nil, err
	}

	for _, co := range cops {
		fi, err := os.Stat(co.srcPath)
		if err != nil {
			return nil, err
		}
		fh, err := os.Open(co.srcPath)
		if err != nil {
			return nil, err
		}
		hdr := &tar.Header{
			Name: co.destPath,
			Mode: int64(fi.Mode()),
			Size: fi.Size(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		_, err = io.Copy(tw, fh)
		if err != nil {
			return nil, err
		}
		// lint made me do it
		_ = fh.Close()
	}
	err = tw.Close()
	return buf, err
}

func (b *buildImage) copyOperations() ([]copyOperation, error) {
	cops := make([]copyOperation, 1)
	cops[0] = copyOperation{
		srcPath:  filepath.Join(b.basePath, "crossplane.yaml"),
		destPath: filepath.Join(DockerTarRoot, "crossplane.yaml"),
	}
	err := filepath.Walk(b.basePath, func(path string, info os.FileInfo, err error) error {
		co, err := b.InclusionFilter(path, info, err)
		if err != nil {
			return err
		}
		if co != nil {
			cops = append(cops, *co)
		}
		return nil
	})
	return cops, err
}

// copyOperation represents a file that should be included in the package
type copyOperation struct {
	// srcPath is the path on the local (builder) filesystem
	srcPath string
	// destPath is the path within the docker 'context' filesystem tar
	destPath string
}

// InclusionFilter can decide whether to include a file based on type, contents, location, and control the destination
type InclusionFilter func(path string, info os.FileInfo, err error) (*copyOperation, error)

// by default everything that is not a directory is included
func buildDefaultInclusionFilter(localBase, dockerBase string) InclusionFilter {
	return func(path string, info os.FileInfo, err error) (*copyOperation, error) {
		if info.IsDir() {
			return nil, nil
		}
		relPath := path[len(localBase):]
		return &copyOperation{
			srcPath:  filepath.Join(localBase, relPath),
			destPath: filepath.Join(dockerBase, relPath),
		}, nil
	}
}

// buildIgnoreFilter assumes the default directory-skipping filter is somewhere in the chain
// so it does not specifically skip directories unless they are in the ignore list
func buildIgnoreFilter(root string, ignorePaths []string, chain InclusionFilter) InclusionFilter {
	absPaths := make([]string, 0)
	for _, p := range ignorePaths {
		ap := filepath.Join(root, p)
		absPaths = append(absPaths, ap)
	}
	return func(path string, info os.FileInfo, err error) (*copyOperation, error) {
		for _, ignore := range absPaths {
			fmt.Println(path + "?" + ignore)
			if strings.HasPrefix(path, ignore) {
				if info.IsDir() {
					fmt.Println("skipping directory (and contents): " + path)
					return nil, filepath.SkipDir
				}
				fmt.Println("skipping file: " + path)
				return nil, nil
			}
		}
		return chain(path, info, err)
	}
}
