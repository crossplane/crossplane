// Package core contains structs and functions to aggregate built-in kube clients for use elsewhere
package core

import (
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis/pkg"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource/xrm"
	"golang.org/x/net/context"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Initializable is an interface for any client that can be initialized
type Initializable interface {
	Initialize(ctx context.Context) error
}

// Clients aggregates the root level of built-in kube clients that we use to initialize our wrapper interfaces
type Clients struct {
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
	Tree      *xrm.Client
}

// NewClients initializes a bundle of built-in kube clients using the given rest config
func NewClients(config *rest.Config) (*Clients, error) {
	// These three clients underlie all of our client wrapper interfaces.
	dynClient, err := makeDynamicClient(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dynamic client")
	}
	disClient, err := makeDiscoveryClient(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create discovery client")
	}
	xrmClient, err := makeXrmClient(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create xrm client")
	}

	return &Clients{
		Dynamic:   dynClient,
		Discovery: disClient,
		Tree:      xrmClient,
	}, nil
}

func makeDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	return dynamic.NewForConfig(config)
}

func makeXrmClient(config *rest.Config) (*xrm.Client, error) {
	c, err := client.New(config, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create controller runtime client")
	}

	_ = pkg.AddToScheme(c.Scheme())

	xrmClient, err := xrm.NewClient(c,
		xrm.WithConnectionSecrets(false),
		xrm.WithConcurrency(5))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create resource tree client")
	}
	return xrmClient, nil
}

func makeDiscoveryClient(config *rest.Config) (discovery.DiscoveryInterface, error) {
	return discovery.NewDiscoveryClientForConfig(config)
}

// InitializeClients initializes a list of clients, collecting all errors
func InitializeClients(ctx context.Context, logger logging.Logger, clients ...Initializable) error {
	var errs []error

	for _, c := range clients {
		clientType := reflect.TypeOf(c).String()
		logger.Debug("Initializing client", "type", clientType)

		if err := c.Initialize(ctx); err != nil {
			logger.Debug("Failed to initialize client", "type", clientType, "error", err)
			errs = append(errs, fmt.Errorf("failed to initialize %s: %w", clientType, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
