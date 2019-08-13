/*
Copyright 2019 The Crossplane Authors.

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

package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	simpleAppFile = `# Human readable title of application.
title: Sample Crossplane Extension

# Markdown description of this entry
description: |
 Markdown describing this sample Crossplane extension project.

# Version of project (optional)
# If omitted the version will be filled with the docker tag
# If set it must match the docker tag
version: 0.0.1

# Maintainer names and emails.
maintainers:
- name: Jared Watts
  email: jared@upbound.io

# Owner names and emails.
owners:
- name: Bassam Tabbara
  email: bassam@upbound.io

# Human readable company name.
company: Upbound

# Keywords that describe this application and help search indexing
keywords:
- "samples"
- "examples"
- "tutorials"

# Links to more information about the application (about page, source code, etc.)
links:
- description: Website
  url: "https://upbound.io"
- description: Source Code
  url: "https://github.com/crossplaneio/sample-extension"

# License SPDX name: https://spdx.org/licenses/
license: Apache-2.0
`

	simpleDeploymentInstallFile = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: crossplane-sample-extension
  labels:
    core.crossplane.io/name: "crossplane-sample-extension"
spec:
  selector:
    matchLabels:
      core.crossplane.io/name: "crossplane-sample-extension"
  replicas: 1
  template:
    metadata:
      name: sample-extension-controller
      labels:
        core.crossplane.io/name: "crossplane-sample-extension"
    spec:
      containers:
      - name: sample-extension-controller
        image: crossplane/sample-extension:latest
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
`

	simpleJobInstallFile = `apiVersion: batch/v1
kind: Job
metadata:
  name: crossplane-sample-install-job
spec:
  completions: 1
  parallelism: 1
  backoffLimit: 4
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: sample-extension-from-job
        image: crossplane/sample-extension-from-job:latest
        args: ["prepare"]
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
`

	simpleDeploymentRBACFile = `rules:
- apiGroups:
  - ""
  resources:
  - secrets
  - serviceaccounts
  - events
  - namespaces
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
`
	simpleJobRBACFile = `rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  - services
  - secrets
  - serviceaccounts
  - events
  - namespaces
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
`

	simpleCRDFile = `apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: mytypes.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Mytype
    listKind: MytypeList
    plural: mytypes
    singular: mytype
  scope: Namespaced
  version: v1alpha1
`

	expectedSimpleDeploymentExtensionOutput = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: mytypes.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Mytype
    listKind: MytypeList
    plural: mytypes
    singular: mytype
  scope: Namespaced
  version: v1alpha1

---
apiVersion: extensions.crossplane.io/v1alpha1
kind: Extension
metadata:
  creationTimestamp: null
spec:
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-extension
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-extension
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              core.crossplane.io/name: crossplane-sample-extension
            name: sample-extension-controller
          spec:
            containers:
            - env:
              - name: POD_NAME
                valueFrom:
                  fieldRef:
                    fieldPath: metadata.name
              - name: POD_NAMESPACE
                valueFrom:
                  fieldRef:
                    fieldPath: metadata.namespace
              image: crossplane/sample-extension:latest
              name: sample-extension-controller
              resources: {}
  customresourcedefinitions:
    owns:
    - apiVersion: samples.upbound.io/v1alpha1
      kind: Mytype
  description: |
    Markdown describing this sample Crossplane extension project.
  icons:
  - base64Data: bW9jay1pY29uLWRh
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  links:
  - description: Website
    url: https://upbound.io
  - description: Source Code
    url: https://github.com/crossplaneio/sample-extension
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  permissions:
    rules:
    - apiGroups:
      - ""
      resources:
      - secrets
      - serviceaccounts
      - events
      - namespaces
      verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  title: Sample Crossplane Extension
  version: 0.0.1
status:
  conditionedStatus: {}
`

	expectedSimpleJobExtensionOutput = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: mytypes.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Mytype
    listKind: MytypeList
    plural: mytypes
    singular: mytype
  scope: Namespaced
  version: v1alpha1

---
apiVersion: extensions.crossplane.io/v1alpha1
kind: Extension
metadata:
  creationTimestamp: null
spec:
  company: Upbound
  controller:
    job:
      name: crossplane-sample-install-job
      spec:
        backoffLimit: 4
        completions: 1
        parallelism: 1
        template:
          metadata:
            creationTimestamp: null
          spec:
            containers:
            - args:
              - prepare
              env:
              - name: POD_NAME
                valueFrom:
                  fieldRef:
                    fieldPath: metadata.name
              - name: POD_NAMESPACE
                valueFrom:
                  fieldRef:
                    fieldPath: metadata.namespace
              image: crossplane/sample-extension-from-job:latest
              name: sample-extension-from-job
              resources: {}
            restartPolicy: Never
  customresourcedefinitions:
    owns:
    - apiVersion: samples.upbound.io/v1alpha1
      kind: Mytype
  description: |
    Markdown describing this sample Crossplane extension project.
  icons:
  - base64Data: bW9jay1pY29uLWRh
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  links:
  - description: Website
    url: https://upbound.io
  - description: Source Code
    url: https://github.com/crossplaneio/sample-extension
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  permissions:
    rules:
    - apiGroups:
      - ""
      resources:
      - configmaps
      - services
      - secrets
      - serviceaccounts
      - events
      - namespaces
      verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  title: Sample Crossplane Extension
  version: 0.0.1
status:
  conditionedStatus: {}
`
)

func TestFindRegistryRoot(t *testing.T) {
	tests := []struct {
		name string
		fs   afero.Fs
		dir  string
		want string
	}{
		{
			name: "EmptyFilesystem",
			fs:   afero.NewMemMapFs(),
			dir:  "/",
			want: "/",
		},
		{
			name: "NoRegistrySubDir",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/b.txt", []byte("file b"), 0644)
				return fs
			}(),
			dir:  "ext-dir",
			want: "ext-dir",
		},
		{
			// registry root actually exists as a subdir underneath the requested dir, this subdir
			// should be returned.  This is a common case for when the registry root has been copied
			// to the requested location.
			name: "RegistrySubDir",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("/ext-dir/.registry", 0755)
				return fs
			}(),
			dir:  "/ext-dir",
			want: "/ext-dir/.registry",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findRegistryRoot(tt.fs, tt.dir)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("findRegistryRoot() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestUnpack(t *testing.T) {
	type want struct {
		output string
		err    error
	}

	tests := []struct {
		name string
		fs   afero.Fs
		root string
		want want
	}{
		{
			// unpack should fail to find the install.yaml file
			name: "EmptyExtensionDir",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: "", err: &os.PathError{Op: "open", Path: "ext-dir/install.yaml", Err: afero.ErrFileNotFound}},
		},
		{
			name: "SimpleDeploymentExtension",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile), 0644)
				afero.WriteFile(fs, "ext-dir/rbac.yaml", []byte(simpleDeploymentRBACFile), 0644)
				crdDir := "ext-dir/resources/samples.upbound.io/mytype/v1alpha1"
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedSimpleDeploymentExtensionOutput, err: nil},
		},
		{
			name: "SimpleJobExtension",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleJobInstallFile), 0644)
				afero.WriteFile(fs, "ext-dir/rbac.yaml", []byte(simpleJobRBACFile), 0644)
				crdDir := "ext-dir/resources/samples.upbound.io/mytype/v1alpha1"
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedSimpleJobExtensionOutput, err: nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doUnpack(tt.fs, tt.root)

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("doUnpack() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.output, got); diff != "" {
				t.Errorf("doUnpack() -want, +got:\n%v", diff)
			}
		})
	}
}

func TestFindResourcesFiles(t *testing.T) {
	type want struct {
		found []string
		err   error
	}

	tests := []struct {
		name         string
		fs           afero.Fs
		resourcesDir string
		want         want
	}{
		{
			// unpack should fail to find the install.yaml file
			name: "EmptyDir",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("resources", 0755)
				return fs
			}(),
			resourcesDir: "resources",
			want:         want{found: []string{}, err: nil},
		},
		{
			name: "MultipleResourcesFilesNamingStyles",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("resources", 0755)
				crdDir1 := "resources/samples.upbound.io/mytype/v1alpha1"
				fs.MkdirAll(crdDir1, 0755)
				crdDir2 := "resources/samples.upbound.io/yourtype/v1alpha1"
				fs.MkdirAll(crdDir2, 0755)
				// write empty CRD files, the content doesn't matter for this test
				afero.WriteFile(fs, filepath.Join(crdDir1, "mytype.v1alpha1.crd.yaml"), []byte{}, 0644) // prefix for the CRD filename using its kind and version
				afero.WriteFile(fs, filepath.Join(crdDir2, "crd.yaml"), []byte{}, 0644)                 // no prefix for the CRD filename, just "crd.yaml"
				afero.WriteFile(fs, filepath.Join(crdDir2, "random-file.txt"), []byte{}, 0644)          // random file that should NOT be found
				return fs
			}(),
			resourcesDir: "resources",
			want: want{
				found: []string{
					// both CRD files should have been found
					"resources/samples.upbound.io/mytype/v1alpha1/mytype.v1alpha1.crd.yaml",
					"resources/samples.upbound.io/yourtype/v1alpha1/crd.yaml",
				},
				err: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findResourcesFiles(tt.fs, tt.resourcesDir)

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("findResourcesFiles() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.found, got); diff != "" {
				t.Errorf("findResourcesFiles() -want, +got:\n%v", diff)
			}
		})
	}
}
