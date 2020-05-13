/*
Copyright 2019 The Crossplane Authors.

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

package unpack

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/pkg/packages"
	"github.com/crossplane/crossplane/pkg/packages/walker"
)

// Command configuration for unpacking packages.
type Command struct {
	Name                      string
	Dir                       string
	OutFile                   string
	PermissionScope           string
	TemplatingControllerImage string
}

// FromKingpin produces a package unpack command from a Kingpin command.
func FromKingpin(cmd *kingpin.CmdClause) *Command {
	c := &Command{Name: cmd.FullCommand()}
	cmd.Flag("content-dir", "The absolute path of the directory that contains the package contents").Required().StringVar(&c.Dir)
	cmd.Flag("outfile", "The file where the YAML Package record and CRD artifacts will be written").StringVar(&c.OutFile)
	cmd.Flag("permission-scope", "The permission-scope that the package must request (Namespaced, Cluster)").Default("Namespaced").EnumVar(&c.PermissionScope, "Namespaced", "Cluster")
	cmd.Flag("templating-controller-image", "The image of the Template Stacks controller").StringVar(&c.TemplatingControllerImage)
	return c
}

// Run the package unpack command.
func (c *Command) Run(log logging.Logger) error {
	outFile := os.Stdout
	if c.OutFile != "" {
		f, err := os.Create(c.OutFile)
		if err != nil {
			return errors.Wrap(err, "Cannot create output file")
		}
		// https://groups.google.com/d/msg/golang-nuts/Hj7-HV-W_iU/ZqlBiz0REpIJ
		defer f.Close() // nolint:errcheck
		outFile = f
	}
	log.Debug("Unpacking package", "to", outFile.Name())

	// TODO(displague) afero.NewBasePathFs could avoid the need to track Base
	fs := afero.NewOsFs()
	rd := &walker.ResourceDir{Base: filepath.Clean(c.Dir), Walker: afero.Afero{Fs: fs}}
	return errors.Wrap(packages.Unpack(rd, outFile, rd.Base, c.PermissionScope, c.TemplatingControllerImage, log), "failed to unpack packages")
}
