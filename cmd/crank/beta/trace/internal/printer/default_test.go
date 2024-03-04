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

package printer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

func TestDefaultPrinter(t *testing.T) {
	type args struct {
		resource *resource.Resource
		wide     bool
	}

	type want struct {
		output string
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		// Test valid resource
		"ResourceWithChildren": {
			reason: "Should print a complex Resource with children.",
			args: args{
				resource: GetComplexResource(),
				wide:     false,
			},
			want: want{
				// Note: Use spaces instead of tabs for indentation
				//nolint:dupword // False positive for 'True True'
				output: `
NAME                                                   SYNCED    READY   STATUS
ObjectStorage/test-resource (default)                  True      True    
└─ XObjectStorage/test-resource-hash                   True      True    
   ├─ Bucket/test-resource-bucket-hash                 True      True    
   │  ├─ User/test-resource-child-1-bucket-hash        True      False   SomethingWrongHappened: ...rure magna. Non cillum id nulla. Anim culpa do duis consectetur.
   │  ├─ User/test-resource-child-mid-bucket-hash      False     True    CantSync: Sync error with bucket child mid
   │  └─ User/test-resource-child-2-bucket-hash        True      False   SomethingWrongHappened: Error with bucket child 2
   │     └─ User/test-resource-child-2-1-bucket-hash   True      -       
   └─ User/test-resource-user-hash                     Unknown   True    
`,
				err: nil,
			},
		},
		"ResourceWithChildrenWide": {
			reason: "Should print a complex Resource with children even in wide.",
			args: args{
				resource: GetComplexResource(),
				wide:     true,
			},
			want: want{
				// Note: Use spaces instead of tabs for indentation
				//nolint:dupword // False positive for 'True True'
				output: `
NAME                                                   RESOURCE   SYNCED    READY   STATUS
ObjectStorage/test-resource (default)                             True      True    
└─ XObjectStorage/test-resource-hash                              True      True    
   ├─ Bucket/test-resource-bucket-hash                 one        True      True    
   │  ├─ User/test-resource-child-1-bucket-hash        two        True      False   SomethingWrongHappened: Error with bucket child 1: Sint eu mollit tempor ad minim do commodo irure. Magna labore irure magna. Non cillum id nulla. Anim culpa do duis consectetur.
   │  ├─ User/test-resource-child-mid-bucket-hash      three      False     True    CantSync: Sync error with bucket child mid
   │  └─ User/test-resource-child-2-bucket-hash        four       True      False   SomethingWrongHappened: Error with bucket child 2
   │     └─ User/test-resource-child-2-1-bucket-hash              True      -       
   └─ User/test-resource-user-hash                                Unknown   True    
`,
				err: nil,
			},
		},
		"PackageWithChildren": {
			reason: "Should print a complex Package with children.",
			args: args{
				resource: GetComplexPackage(),
			},
			want: want{
				// Note: Use spaces instead of tabs for indentation
				//nolint:dupword // False positive for 'True True'
				output: `
NAME                                                                         VERSION   INSTALLED   HEALTHY   STATE    STATUS                                                                                              
Configuration/platform-ref-aws                                               v0.9.0    True        True      -        HealthyPackageRevision                                                                              
├─ ConfigurationRevision/platform-ref-aws-9ad7b5db2899                       v0.9.0    True        True      Active   HealthyPackageRevision                                                                              
└─ Configuration/upbound-configuration-aws-network                           v0.7.0    True        True      -        HealthyPackageRevision                                                                              
   ├─ ConfigurationRevision/upbound-configuration-aws-network-97be9100cfe1   v0.7.0    True        True      Active   HealthyPackageRevision                                                                              
   └─ Provider/upbound-provider-aws-ec2                                      v0.47.0   True        Unknown   -        UnknownPackageRevisionHealth: ...der-helm xpkg.upbound.io/crossplane-contrib/provider-kubernetes]   
      ├─ ProviderRevision/upbound-provider-aws-ec2-9ad7b5db2899              v0.47.0   True        False     Active   UnhealthyPackageRevision: ...ider package deployment has no condition of type "Available" yet       
      └─ Provider/upbound-provider-aws-something                             v0.47.0   True        -         -        ActivePackageRevision                                                                               
`,
				err: nil,
			},
		},
		"PackageWithChildrenWide": {
			reason: "Should print a complex Package with children.",
			args: args{
				resource: GetComplexPackage(),
				wide:     true,
			},
			want: want{
				// Note: Use spaces instead of tabs for indentation
				//nolint:dupword // False positive for 'True True'
				output: `
NAME                                                                         PACKAGE                                             VERSION   INSTALLED   HEALTHY   STATE    STATUS                                                                                                                                                                                                    
Configuration/platform-ref-aws                                               xpkg.upbound.io/upbound/platform-ref-aws            v0.9.0    True        True      -        HealthyPackageRevision                                                                                                                                                                                    
├─ ConfigurationRevision/platform-ref-aws-9ad7b5db2899                       xpkg.upbound.io/upbound/platform-ref-aws            v0.9.0    True        True      Active   HealthyPackageRevision                                                                                                                                                                                    
└─ Configuration/upbound-configuration-aws-network                           xpkg.upbound.io/upbound/configuration-aws-network   v0.7.0    True        True      -        HealthyPackageRevision                                                                                                                                                                                    
   ├─ ConfigurationRevision/upbound-configuration-aws-network-97be9100cfe1   xpkg.upbound.io/upbound/configuration-aws-network   v0.7.0    True        True      Active   HealthyPackageRevision                                                                                                                                                                                    
   └─ Provider/upbound-provider-aws-ec2                                      xpkg.upbound.io/upbound/provider-aws-ec2            v0.47.0   True        Unknown   -        UnknownPackageRevisionHealth: cannot resolve package dependencies: incompatible dependencies: [xpkg.upbound.io/crossplane-contrib/provider-helm xpkg.upbound.io/crossplane-contrib/provider-kubernetes]   
      ├─ ProviderRevision/upbound-provider-aws-ec2-9ad7b5db2899              xpkg.upbound.io/upbound/provider-aws-ec2            v0.47.0   True        False     Active   UnhealthyPackageRevision: post establish runtime hook failed for package: provider package deployment has no condition of type "Available" yet                                                            
      └─ Provider/upbound-provider-aws-something                             xpkg.upbound.io/upbound/provider-aws-something      v0.47.0   True        -         -        ActivePackageRevision                                                                                                                                                                                     
`,
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := DefaultPrinter{
				wide: tc.args.wide,
			}
			var buf bytes.Buffer
			err := p.Print(&buf, tc.args.resource)
			got := buf.String()

			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nCliTableAddResource(): -want, +got:\n%s", tc.reason, diff)
			}
			// Check table
			if diff := cmp.Diff(strings.TrimSpace(tc.want.output), strings.TrimSpace(got)); diff != "" {
				t.Errorf("%s\nCliTableAddResource(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
