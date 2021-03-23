// +build e2e

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
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	typedclient "github.com/crossplane/crossplane/pkg/client/clientset/versioned/typed/pkg/v1beta1"
)

const (
	configName    = "getting-started-with-aws"
	configPackage = "registry.upbound.io/xp/getting-started-with-aws:latest"
)

func TestDependencies(t *testing.T) {
	cases := map[string]struct {
		reason string
		body   func() error
	}{
		"ResolveDependencies": {
			reason: "Should successfully resolve dependencies for a package.",
			body: func() error {
				ctx := context.Background()
				c := typedclient.NewForConfigOrDie(ctrl.GetConfigOrDie())
				cr := &v1beta1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Name: configName,
					},
					Spec: v1beta1.ConfigurationSpec{
						PackageSpec: v1beta1.PackageSpec{
							Package: configPackage,
						},
					},
				}
				_, err := c.Configurations().Create(ctx, cr, metav1.CreateOptions{})
				if err != nil {
					return err
				}
				if err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
					l, err := c.Providers().List(ctx, metav1.ListOptions{})
					if err != nil {
						return false, err
					}
					if len(l.Items) != 1 {
						return false, nil
					}
					for _, p := range l.Items {
						if p.GetCondition(v1beta1.TypeInstalled).Status != corev1.ConditionTrue {
							return false, nil
						}
						if p.GetCondition(v1beta1.TypeHealthy).Status != corev1.ConditionTrue {
							return false, nil
						}
					}
					return true, nil
				}); err != nil {
					return err
				}
				if err := c.Configurations().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
					return err
				}
				if err := wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
					_, err := c.Configurations().Get(ctx, configName, metav1.GetOptions{})
					return kerrors.IsNotFound(err), nil
				}); err != nil {
					return err
				}
				if err := c.Providers().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
					return err
				}
				return wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
					l, err := c.Providers().List(ctx, metav1.ListOptions{})
					if err != nil {
						return false, nil
					}
					return len(l.Items) == 0, nil
				})
			},
		},
		"DoNotResolveDependencies": {
			reason: "Should not resolve dependencies for a package if skip is true.",
			body: func() error {
				ctx := context.Background()
				c := typedclient.NewForConfigOrDie(ctrl.GetConfigOrDie())
				cr := &v1beta1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Name: configName,
					},
					Spec: v1beta1.ConfigurationSpec{
						PackageSpec: v1beta1.PackageSpec{
							Package:                  configPackage,
							SkipDependencyResolution: pointer.BoolPtr(true),
						},
					},
				}
				if _, err := c.Configurations().Create(ctx, cr, metav1.CreateOptions{}); err != nil {
					return err
				}
				if err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
					config, err := c.Configurations().Get(ctx, configName, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					if config.GetCondition(v1beta1.TypeInstalled).Status != corev1.ConditionTrue {
						return false, nil
					}
					if config.GetCondition(v1beta1.TypeHealthy).Status != corev1.ConditionTrue {
						return false, nil
					}
					l, err := c.Providers().List(ctx, metav1.ListOptions{})
					if err != nil {
						return false, err
					}
					if len(l.Items) != 0 {
						return false, errors.Errorf("unexpected number of providers %d", len(l.Items))
					}
					return false, nil
				}); err != nil && err != wait.ErrWaitTimeout {
					return err
				}
				if err := c.Configurations().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
					return err
				}
				return wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
					_, err := c.Configurations().Get(ctx, configName, metav1.GetOptions{})
					return kerrors.IsNotFound(err), nil
				})
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := tc.body(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
