/*
Copyright 2025 The Crossplane Authors.

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

// A composition function that composes a ConfigMap per name in a sequence, and
// declares ordering constraints between them.
//
// It exists to exercise composed resource ordering end to end. No published
// function can, because they were all compiled before the dependencies field
// existed - which is the compatibility problem the feature is about.
//
// Input, in the Composition pipeline step:
//
//	apiVersion: ordering.fn.crossplane.io/v1alpha1
//	kind: Input
//	sequence: [first, second, third]
//
// Each name becomes a ConfigMap, and each resource depends on every name before
// it in the sequence. A resource is reported ready once it shows up in observed
// state, which is what lets the ordering be observed: Crossplane won't create
// the second ConfigMap until it has seen the first.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

type edge struct {
	Resource  string `json:"resource"`
	DependsOn string `json:"dependsOn"`
	// CreateBeforeDestroy lets Resource be created without waiting for
	// DependsOn to be deleted.
	CreateBeforeDestroy bool `json:"createBeforeDestroy"`
}

type input struct {
	// Sequence is shorthand for a chain: each name depends on every name
	// before it. Names in a sequence are also composed.
	Sequence []string `json:"sequence"`

	// Resources are composed without implying any ordering. Use with Edges to
	// describe an arbitrary graph rather than a chain.
	Resources []string `json:"resources"`

	// Edges are explicit ordering constraints over Resources.
	Edges []edge `json:"edges"`

	// ReadyAfter composes NopResources that report Ready this long after
	// they're created, instead of ConfigMaps. It makes each ordering wave take
	// a controlled, observable amount of time rather than completing in
	// milliseconds. Readiness is then read from the resource itself, the way a
	// real function would, rather than assumed on first sight.
	ReadyAfter string `json:"readyAfter"`

	// DeleteAfter composes NopResources whose deletion takes this long, so
	// that a teardown wave is observable rather than completing before
	// anything can look at it. Implies NopResources, like ReadyAfter.
	DeleteAfter string `json:"deleteAfter"`

	// DeleteError makes the resources named in DeleteErrorOn refuse to delete,
	// with this message. Teardown then blocks on them until something clears
	// the field, which is how a test exercises a composed resource that cannot
	// be deleted - a provider that can't reach its API, or an external
	// resource something else still holds.
	DeleteError string `json:"deleteError"`

	// DeleteErrorOn names the resources DeleteError applies to. Leave empty to
	// apply it to none.
	DeleteErrorOn []string `json:"deleteErrorOn"`

	// Requires declares resources the pipeline needs but doesn't compose.
	Requires []requirement `json:"requires"`

	// RequiredEdges make a composed resource wait on a required resource.
	RequiredEdges []requiredEdge `json:"requiredEdges"`

	// Remove names from desired state, to exercise the delete path while the
	// XR is alive.
	Remove []string `json:"remove"`
}

// A requirement is a resource the pipeline needs but doesn't compose.
type requirement struct {
	Name       string `json:"name"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	MatchName  string `json:"matchName"`
	Namespace  string `json:"namespace"`
}

// A requiredEdge makes a composed resource depend on a required resource.
type requiredEdge struct {
	Resource    string `json:"resource"`
	Requirement string `json:"requirement"`
}

// names returns every resource this input composes.
func (i *input) names() []string {
	out := append([]string{}, i.Sequence...)

	seen := map[string]bool{}
	for _, n := range out {
		seen[n] = true
	}

	for _, n := range i.Resources {
		if !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}

	return out
}

type function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log *slog.Logger
}

func (f *function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	in := &input{}

	if s := req.GetInput(); s != nil {
		b, err := s.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("cannot marshal input: %w", err)
		}

		if err := json.Unmarshal(b, in); err != nil {
			return nil, fmt.Errorf("cannot unmarshal input: %w", err)
		}
	}

	xr := req.GetObserved().GetComposite().GetResource()
	ns := xr.GetFields()["metadata"].GetStructValue().GetFields()["namespace"].GetStringValue()

	remove := map[string]bool{}
	for _, n := range in.Remove {
		remove[n] = true
	}

	rsp := &fnv1.RunFunctionResponse{
		Meta:    &fnv1.ResponseMeta{Tag: req.GetMeta().GetTag(), Ttl: durationpb.New(60_000_000_000)},
		Desired: &fnv1.State{Composite: req.GetDesired().GetComposite(), Resources: map[string]*fnv1.Resource{}},
		// Always return the full set, including edges for resources we're
		// removing. Dropping those is the mistake the retention rule guards
		// against, and this function shouldn't rely on that guard.
		Dependencies: &fnv1.Dependencies{},
	}

	for _, name := range in.names() {
		if !remove[name] {
			var (
				res *structpb.Struct
				err error
			)

			if in.ReadyAfter != "" || in.DeleteAfter != "" || in.DeleteError != "" {
				res, err = nopResource(name, ns, in.ReadyAfter, in.DeleteAfter, deleteErrorFor(in, name))
			} else {
				res, err = configMap(name, ns)
			}

			if err != nil {
				return nil, err
			}

			rsp.Desired.Resources[name] = &fnv1.Resource{
				Resource: res,
				Ready:    readiness(req, name, in.ReadyAfter != ""),
			}
		}
	}

	// A sequence is shorthand for "each name depends on every name before it",
	// matching function-sequencer's semantics.
	for i, name := range in.Sequence {
		for _, before := range in.Sequence[:i] {
			rsp.Dependencies.Items = append(rsp.Dependencies.Items, &fnv1.Dependency{
				Resource:  name,
				DependsOn: &fnv1.Dependency_ComposedResource{ComposedResource: before},
			})
		}
	}

	// Ask Crossplane to fetch the resources we require. It returns them in the
	// next request's required_resources, and an edge can then wait on them.
	if len(in.Requires) > 0 {
		rsp.Requirements = &fnv1.Requirements{Resources: map[string]*fnv1.ResourceSelector{}}

		for _, r := range in.Requires {
			sel := &fnv1.ResourceSelector{
				ApiVersion: r.APIVersion,
				Kind:       r.Kind,
				Match:      &fnv1.ResourceSelector_MatchName{MatchName: r.MatchName},
			}

			if r.Namespace != "" {
				sel.Namespace = &r.Namespace
			}

			rsp.Requirements.Resources[r.Name] = sel
		}
	}

	for _, e := range in.RequiredEdges {
		rsp.Dependencies.Items = append(rsp.Dependencies.Items, &fnv1.Dependency{
			Resource: e.Resource,
			DependsOn: &fnv1.Dependency_RequiredResource{
				RequiredResource: &fnv1.RequiredResourceDependency{RequirementName: e.Requirement},
			},
		})
	}

	for _, e := range in.Edges {
		// The protocol carries the create-before-destroy choice as a
		// DependencyLifecycle enum rather than a boolean, so that further
		// lifecycle policies can be added without a second flag.
		lifecycle := fnv1.DependencyLifecycle_DEPENDENCY_LIFECYCLE_UNSPECIFIED
		if e.CreateBeforeDestroy {
			lifecycle = fnv1.DependencyLifecycle_DEPENDENCY_LIFECYCLE_CREATE_BEFORE_DESTROY
		}

		rsp.Dependencies.Items = append(rsp.Dependencies.Items, &fnv1.Dependency{
			Resource:  e.Resource,
			DependsOn: &fnv1.Dependency_ComposedResource{ComposedResource: e.DependsOn},
			Lifecycle: lifecycle,
		})
	}

	f.log.Info("ran function",
		"desired", len(rsp.GetDesired().GetResources()),
		"dependencies", len(rsp.GetDependencies().GetItems()),
		"required", len(req.GetRequiredResources()))

	return rsp, nil
}

// readiness reports whether a composed resource should count as ready.
//
// For NopResources we read the resource's own Ready condition, which
// provider-nop flips after the configured delay. For ConfigMaps, which have no
// conditions, existence is all the readiness there is.
func readiness(req *fnv1.RunFunctionRequest, name string, fromCondition bool) fnv1.Ready {
	o, ok := req.GetObserved().GetResources()[name]
	if !ok {
		return fnv1.Ready_READY_FALSE
	}

	if !fromCondition {
		return fnv1.Ready_READY_TRUE
	}

	conds := o.GetResource().GetFields()["status"].GetStructValue().GetFields()["conditions"].GetListValue()
	for _, c := range conds.GetValues() {
		f := c.GetStructValue().GetFields()
		if f["type"].GetStringValue() == "Ready" && f["status"].GetStringValue() == "True" {
			return fnv1.Ready_READY_TRUE
		}
	}

	return fnv1.Ready_READY_FALSE
}

// deleteErrorFor returns the delete error to give a composed resource, which
// is set only for the resources DeleteErrorOn names.
func deleteErrorFor(in *input, name string) string {
	if slices.Contains(in.DeleteErrorOn, name) {
		return in.DeleteError
	}

	return ""
}

// nopResource returns a NopResource that reports Ready after readyAfter, and
// whose deletion takes deleteAfter. Either may be empty. Together they let a
// test observe creation and teardown over seconds rather than milliseconds.
func nopResource(name, namespace, readyAfter, deleteAfter, deleteError string) (*structpb.Struct, error) {
	fp := map[string]any{}

	if readyAfter != "" {
		fp["conditionAfter"] = []any{
			map[string]any{"time": "0s", "conditionType": "Ready", "conditionStatus": "False"},
			map[string]any{"time": readyAfter, "conditionType": "Ready", "conditionStatus": "True"},
		}
	}

	if deleteAfter != "" {
		fp["deleteAfter"] = deleteAfter
	}

	if deleteError != "" {
		fp["deleteError"] = deleteError
	}

	m := map[string]any{
		"apiVersion": "nop.crossplane.io/v1alpha1",
		"kind":       "NopResource",
		"metadata":   map[string]any{"namespace": namespace},
		"spec":       map[string]any{"forProvider": fp},
	}

	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, fmt.Errorf("cannot build NopResource %q: %w", name, err)
	}

	return s, nil
}

// configMap returns a ConfigMap named after the composed resource.
func configMap(name, namespace string) (*structpb.Struct, error) {
	m := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"namespace": namespace},
		"data":       map[string]any{"name": name},
	}

	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, fmt.Errorf("cannot build ConfigMap %q: %w", name, err)
	}

	return s, nil
}

// serverCreds builds mTLS credentials from a directory holding tls.crt,
// tls.key and ca.crt, requiring and verifying the client's certificate the way
// Crossplane's own function server does.
func serverCreds(dir string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key"))
	if err != nil {
		return nil, fmt.Errorf("cannot load key pair: %w", err)
	}

	ca, err := os.ReadFile(filepath.Join(dir, "ca.crt")) //nolint:gosec // The path is operator-supplied, in a throwaway test cluster.
	if err != nil {
		return nil, fmt.Errorf("cannot read CA: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("cannot parse CA from %s", dir)
	}

	return credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}), nil
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	addr := os.Getenv("ADDRESS")
	if addr == "" {
		addr = ":9443"
	}

	lc := &net.ListenConfig{}

	lis, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		log.Error("cannot listen", "error", err)
		os.Exit(1)
	}

	// Crossplane dials functions over mTLS. Certs are optional here so the
	// function can also be run without them for quick local experiments.
	opts := []grpc.ServerOption{}

	if dir := os.Getenv("TLS_SERVER_CERTS_DIR"); dir != "" {
		creds, err := serverCreds(dir)
		if err != nil {
			log.Error("cannot load TLS certificates", "error", err)
			os.Exit(1)
		}

		opts = append(opts, grpc.Creds(creds))
	}

	s := grpc.NewServer(opts...)
	fnv1.RegisterFunctionRunnerServiceServer(s, &function{log: log})

	log.Info("listening", "address", addr)

	if err := s.Serve(lis); err != nil {
		log.Error("cannot serve", "error", err)
		os.Exit(1)
	}
}
