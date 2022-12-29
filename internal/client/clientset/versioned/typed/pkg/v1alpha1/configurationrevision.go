/*
Copyright 2021 The Crossplane Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	scheme "github.com/crossplane/crossplane/internal/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ConfigurationRevisionsGetter has a method to return a ConfigurationRevisionInterface.
// A group's client should implement this interface.
type ConfigurationRevisionsGetter interface {
	ConfigurationRevisions() ConfigurationRevisionInterface
}

// ConfigurationRevisionInterface has methods to work with ConfigurationRevision resources.
type ConfigurationRevisionInterface interface {
	Create(ctx context.Context, configurationRevision *v1alpha1.ConfigurationRevision, opts v1.CreateOptions) (*v1alpha1.ConfigurationRevision, error)
	Update(ctx context.Context, configurationRevision *v1alpha1.ConfigurationRevision, opts v1.UpdateOptions) (*v1alpha1.ConfigurationRevision, error)
	UpdateStatus(ctx context.Context, configurationRevision *v1alpha1.ConfigurationRevision, opts v1.UpdateOptions) (*v1alpha1.ConfigurationRevision, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ConfigurationRevision, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ConfigurationRevisionList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ConfigurationRevision, err error)
	ConfigurationRevisionExpansion
}

// configurationRevisions implements ConfigurationRevisionInterface
type configurationRevisions struct {
	client rest.Interface
}

// newConfigurationRevisions returns a ConfigurationRevisions
func newConfigurationRevisions(c *PkgV1alpha1Client) *configurationRevisions {
	return &configurationRevisions{
		client: c.RESTClient(),
	}
}

// Get takes name of the configurationRevision, and returns the corresponding configurationRevision object, and an error if there is any.
func (c *configurationRevisions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ConfigurationRevision, err error) {
	result = &v1alpha1.ConfigurationRevision{}
	err = c.client.Get().
		Resource("configurationrevisions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ConfigurationRevisions that match those selectors.
func (c *configurationRevisions) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ConfigurationRevisionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ConfigurationRevisionList{}
	err = c.client.Get().
		Resource("configurationrevisions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested configurationRevisions.
func (c *configurationRevisions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("configurationrevisions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a configurationRevision and creates it.  Returns the server's representation of the configurationRevision, and an error, if there is any.
func (c *configurationRevisions) Create(ctx context.Context, configurationRevision *v1alpha1.ConfigurationRevision, opts v1.CreateOptions) (result *v1alpha1.ConfigurationRevision, err error) {
	result = &v1alpha1.ConfigurationRevision{}
	err = c.client.Post().
		Resource("configurationrevisions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(configurationRevision).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a configurationRevision and updates it. Returns the server's representation of the configurationRevision, and an error, if there is any.
func (c *configurationRevisions) Update(ctx context.Context, configurationRevision *v1alpha1.ConfigurationRevision, opts v1.UpdateOptions) (result *v1alpha1.ConfigurationRevision, err error) {
	result = &v1alpha1.ConfigurationRevision{}
	err = c.client.Put().
		Resource("configurationrevisions").
		Name(configurationRevision.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(configurationRevision).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *configurationRevisions) UpdateStatus(ctx context.Context, configurationRevision *v1alpha1.ConfigurationRevision, opts v1.UpdateOptions) (result *v1alpha1.ConfigurationRevision, err error) {
	result = &v1alpha1.ConfigurationRevision{}
	err = c.client.Put().
		Resource("configurationrevisions").
		Name(configurationRevision.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(configurationRevision).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the configurationRevision and deletes it. Returns an error if one occurs.
func (c *configurationRevisions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("configurationrevisions").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *configurationRevisions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("configurationrevisions").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched configurationRevision.
func (c *configurationRevisions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ConfigurationRevision, err error) {
	result = &v1alpha1.ConfigurationRevision{}
	err = c.client.Patch(pt).
		Resource("configurationrevisions").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
