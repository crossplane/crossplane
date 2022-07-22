package main

import (
	"context"
	"fmt"
	"github.com/GoogleCloudPlatform/declarative-resource-client-library/dcl"
	"github.com/GoogleCloudPlatform/declarative-resource-client-library/services/google/compute"
	"time"
)

func main() {
	cl := compute.NewClient(dcl.NewConfig(
		dcl.WithCredentialsFile("/Users/monus/go/src/github.com/crossplane/crossplane/crossplane-gcp-provider-key.json"),
		dcl.WithLogger(dcl.DefaultLogger(dcl.Error)),
	))
	ctx := context.TODO()

	// See the API schema of Network here:
	// https://github.com/GoogleCloudPlatform/declarative-resource-client-library/blob/main/services/google/compute/network.yaml

	in := &compute.Network{
		Project:               dcl.String("crossplane-playground"),
		Name:                  dcl.String("muvaf-testing-dcl"),
		AutoCreateSubnetworks: dcl.Bool(false),
	}
	start := time.Now()
	fmt.Println("Started apply to create...")
	out, err := cl.ApplyNetwork(ctx, in,
		dcl.WithLifecycleParam(dcl.BlockDestruction),
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Completed apply in %f seconds.\n\n", time.Now().Sub(start).Seconds())
	fmt.Printf("Response object:\n%s\n\n", out.String())

	in.RoutingConfig = &compute.NetworkRoutingConfig{
		// No metadata for enums, it's buried in the validation function.
		RoutingMode: compute.NetworkRoutingConfigRoutingModeEnumRef("GLOBAL"),
	}

	start = time.Now()
	fmt.Println("Started apply to update...")
	out, err = cl.ApplyNetwork(ctx, in,
		dcl.WithLifecycleParam(dcl.BlockDestruction),
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Completed apply in %f seconds.\n\n", time.Now().Sub(start).Seconds())
	fmt.Printf("Response object:\n%s\n\n", out.String())

	start = time.Now()
	fmt.Println("Started to delete...")
	err = cl.DeleteNetwork(ctx, in)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Completed delete in %f seconds.\n\n", time.Now().Sub(start).Seconds())
}
