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

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type MockCache struct {
	cache.Cache

	MockStart func(stop context.Context) error
}

func (c *MockCache) Start(stop context.Context) error {
	return c.MockStart(stop)
}

type MockController struct {
	controller.Controller

	MockStart func(stop context.Context) error
	MockWatch func(s source.Source, h handler.EventHandler, p ...predicate.Predicate) error
}

func (c *MockController) Start(stop context.Context) error {
	return c.MockStart(stop)
}

func (c *MockController) Watch(s source.Source, h handler.EventHandler, p ...predicate.Predicate) error {
	return c.MockWatch(s, h, p...)
}

func TestEngine(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		name string
		o    controller.Options
		w    []Watch
	}
	type want struct {
		err   error
		crash error
	}
	cases := map[string]struct {
		reason string
		e      *Engine
		args   args
		want   want
	}{
		"NewCacheError": {
			reason: "Errors creating a new cache should be returned",
			e: NewEngine(&fake.Manager{},
				WithNewCacheFn(func(*rest.Config, cache.Options) (cache.Cache, error) { return nil, errBoom }),
			),
			args: args{
				name: "coolcontroller",
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateCache),
			},
		},
		"NewControllerError": {
			reason: "Errors creating a new controller should be returned",
			e: NewEngine(&fake.Manager{},
				WithNewCacheFn(func(*rest.Config, cache.Options) (cache.Cache, error) { return nil, nil }),
				WithNewControllerFn(func(string, manager.Manager, controller.Options) (controller.Controller, error) { return nil, errBoom }),
			),
			args: args{
				name: "coolcontroller",
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateController),
			},
		},
		"WatchError": {
			reason: "Errors adding a watch should be returned",
			e: NewEngine(&fake.Manager{},
				WithNewCacheFn(func(*rest.Config, cache.Options) (cache.Cache, error) { return nil, nil }),
				WithNewControllerFn(func(string, manager.Manager, controller.Options) (controller.Controller, error) {
					c := &MockController{MockWatch: func(source.Source, handler.EventHandler, ...predicate.Predicate) error { return errBoom }}
					return c, nil
				}),
			),
			args: args{
				name: "coolcontroller",
				w:    []Watch{For(&fake.Managed{}, nil)},
			},
			want: want{
				err: errors.Wrap(errBoom, errWatch),
			},
		},
		"CacheCrashError": {
			reason: "Errors starting or running a cache should be returned",
			e: NewEngine(&fake.Manager{},
				WithNewCacheFn(func(*rest.Config, cache.Options) (cache.Cache, error) {
					c := &MockCache{MockStart: func(stop context.Context) error { return errBoom }}
					return c, nil
				}),
				WithNewControllerFn(func(string, manager.Manager, controller.Options) (controller.Controller, error) {
					c := &MockController{MockStart: func(stop context.Context) error {
						return nil
					}}
					return c, nil
				}),
			),
			args: args{
				name: "coolcontroller",
			},
			want: want{
				crash: errors.Wrap(errBoom, errCrashCache),
			},
		},
		"ControllerCrashError": {
			reason: "Errors starting or running a controller should be returned",
			e: NewEngine(&fake.Manager{},
				WithNewCacheFn(func(*rest.Config, cache.Options) (cache.Cache, error) {
					c := &MockCache{MockStart: func(stop context.Context) error {
						return nil
					}}
					return c, nil
				}),
				WithNewControllerFn(func(string, manager.Manager, controller.Options) (controller.Controller, error) {
					c := &MockController{MockStart: func(stop context.Context) error {
						return errBoom
					}}
					return c, nil
				}),
			),
			args: args{
				name: "coolcontroller",
			},
			want: want{
				crash: errors.Wrap(errBoom, errCrashController),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.e.Start(tc.args.name, tc.args.o, tc.args.w...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Give the goroutines a little time to return an error. If this
			// becomes flaky or time consuming we could use a ticker instead.
			time.Sleep(100 * time.Millisecond)

			tc.e.Stop(tc.args.name)
			if diff := cmp.Diff(tc.want.crash, tc.e.Err(tc.args.name), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Err(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
