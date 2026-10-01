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

// Package main implements e2e-function-revision-id, a minimal composition
// function used by Crossplane's e2e tests to report which function revision
// served a request.
package main

import (
	"github.com/alecthomas/kong"

	function "github.com/crossplane/function-sdk-go"
)

// CLI of this function.
type CLI struct {
	Debug bool `help:"Emit debug logs in addition to info logs."  short:"d"`

	Network string `default:"tcp"    help:"Network on which to listen for gRPC connections."`
	Address string `default:":9443"  help:"Address at which to listen for gRPC connections."`

	TLSCertsDir string `env:"TLS_SERVER_CERTS_DIR"  help:"Directory containing server certs (tls.key, tls.crt) and the CA used to verify clients (ca.crt)"`
	Insecure    bool   `help:"Run without mTLS credentials. If you supply this flag --tls-certs-dir will be ignored."`
}

// Run this function.
func (c *CLI) Run() error {
	return function.Serve(&Function{},
		function.Listen(c.Network, c.Address),
		function.MTLSCertificates(c.TLSCertsDir),
		function.Insecure(c.Insecure))
}

func main() {
	ctx := kong.Parse(&CLI{}, kong.Description("A Crossplane composition function that reports its revision identity."))
	ctx.FatalIfErrorf(ctx.Run())
}
