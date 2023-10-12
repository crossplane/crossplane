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

// Package xpkg contains the Crossplane packaging commands.
package xpkg

// Cmd contains commands for interacting with xpkgs.
// TODO(lsviben) add the rest of the commands from up (batch, xpextract).
type Cmd struct {
	Build buildCmd `cmd:"" help:"Build a package, by default from the current directory."`
	Push  pushCmd  `cmd:"" help:"Push Crossplane packages."`
}

// Help prints out the help for the xpkg command.
func (c *Cmd) Help() string {
	return `
A Crossplane package is an opinionated OCI image that contains an additional layer 
holding meta information to drive the Crossplane package manager. The package manager
uses this information to install packages into a Crossplane instance.

Furthermore, a Crossplane package may contain meta information that describes
how to represent the package in a user interface. This information is used by
the Upbound marketplace to display packages and their contents. See the xpkg
reference document for more information.

There are different kinds of Crossplane packages, each with a different set of
meta information and files in the additional layer. The following kinds are 
currently supported:

- **Provider**: A Crossplane package that contains a Crossplane provider. The layer
  contains a crossplane.yaml file with a "meta.pkg.crossplane.io/v1alpha1"
  kind "Provider" manifest, and optionally CRD manifest.
- **Configuration**: A Crossplane package that contains a Crossplane configuration,
  with a "meta.pkg.crossplane.io/v1" kind "Configuration" manifest in crossplane.yaml.
- **Function**: A crossplane package that contains a Crossplane function, with a
  "meta.pkg.crossplane.io/v1beta1" kind "Function" manifest in crossplane.yaml.
- in newer versions of Crossplane, more kinds will be supported.

For more detailed information on Crossplane packages, see

  https://docs.crossplane.io/latest/concepts/packages/#building-a-package

Even more details can be found in the xpkg reference document.`
}
