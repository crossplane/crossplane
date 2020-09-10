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

package provider

// Cmd is the root command for Providers.
type Cmd struct {
	Get    GetCmd    `cmd:"" help:"Get installed Providers."`
	Create CreateCmd `cmd:"" help:"Create a Provider."`
}

// Run runs the Provider command.
func (r *Cmd) Run() error {
	return nil
}

// GetCmd gets one or more Providers in the cluster.
type GetCmd struct {
	Name string `arg:"" optional:"" name:"name" help:"Name of Provider."`

	Revisions bool `short:"r" help:"List revisions for each Provider."`
}

// Run the Get command.
func (b *GetCmd) Run() error {
	return nil
}

// CreateCmd creates a Provider in the cluster.
type CreateCmd struct {
	Name    string `arg:"" name:"name" help:"Name of Provider."`
	Package string `arg:"" name:"package" help:"Image containing Provider package."`

	History  int  `help:"Revision history limit."`
	Activate bool `short:"a" help:"Enable automatic revision activation policy."`
}

// Run the CreateCmd.
func (p *CreateCmd) Run() error {
	return nil
}
