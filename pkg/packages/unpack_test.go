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

package packages

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/packages/walker"
)

const (
	simpleCrdDir = "ext-dir/resources/samples.upbound.io/mytype/v1alpha1"

	simpleGroupFile = `title: Group Title
overviewShort: Group Short Overview
overview: Group Overview
readme: Group Readme
category: Group Category
`

	simpleResourceFile = `id: mytype
title: Resource Title
titlePlural: Resources Title
overviewShort: Resource Short Overview
overview: Resource Overview
readme: Resource Readme
category: Resource Category
`

	expectedComplexDeploymentPackageOutput = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/group-category: Group Category
    packages.crossplane.io/group-overview: Group Overview
    packages.crossplane.io/group-overview-short: Group Short Overview
    packages.crossplane.io/group-readme: Group Readme
    packages.crossplane.io/group-title: Group Title
    packages.crossplane.io/icon-data-uri: data:image/svg+xml;base64,bW9jay1pY29uLWRhdGEtc3Zn
    packages.crossplane.io/package-title: Sample Crossplane Package
    packages.crossplane.io/ui-schema: |-
      configSections:
      - title: group Title
        description: group Description
      ---
      configSections:
      - title: sibling Title
        description: sibling Description
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: namespace
  name: siblings.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Sibling
    listKind: SiblingList
    plural: siblings
    singular: sibling
  scope: Namespaced
  version: v1alpha1
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    packages.crossplane.io/package-title: Sample Crossplane Package
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: namespace
  name: secondcousins.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Secondcousin
    listKind: SecondcousinList
    plural: secondcousins
    singular: secondcousin
  scope: Namespaced
  subresources:
    scale:
      specReplicasPath: ""
      statusReplicasPath: ""
    status: {}
  version: v1alpha1
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/group-category: Group Category
    packages.crossplane.io/group-overview: Group Overview
    packages.crossplane.io/group-overview-short: Group Short Overview
    packages.crossplane.io/group-readme: Group Readme
    packages.crossplane.io/group-title: Group Title
    packages.crossplane.io/icon-data-uri: data:image/svg+xml;base64,bW9jay1pY29uLWRhdGEtc3Zn
    packages.crossplane.io/package-title: Sample Crossplane Package
    packages.crossplane.io/resource-category: Resource Category
    packages.crossplane.io/resource-overview: Resource Overview
    packages.crossplane.io/resource-overview-short: Resource Short Overview
    packages.crossplane.io/resource-readme: Resource Readme
    packages.crossplane.io/resource-title: Resource Title
    packages.crossplane.io/resource-title-plural: Resources Title
    packages.crossplane.io/ui-schema: |-
      configSections:
      - title: group Title
        description: group Description
      ---
      configSections:
      - title: sibling Title
        description: sibling Description
      ---
      configSections:
      - title: kind Title
        description: kind Description
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: namespace
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
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/group-category: Group Category
    packages.crossplane.io/group-overview: Group Overview
    packages.crossplane.io/group-overview-short: Group Short Overview
    packages.crossplane.io/group-readme: Group Readme
    packages.crossplane.io/group-title: Group Title
    packages.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    packages.crossplane.io/package-title: Sample Crossplane Package
    packages.crossplane.io/ui-schema: |-
      configSections:
      - title: group Title
        description: group Description
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: namespace
  name: cousins.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Cousin
    listKind: CousinList
    plural: cousins
    singular: cousin
  scope: Namespaced
  version: v1alpha1
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: packages.crossplane.io/v1alpha1
kind: Package
metadata:
  creationTimestamp: null
spec:
  category: Category
  company: Upbound
  controller:
    deployment:
      name: ""
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-package
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              core.crossplane.io/name: crossplane-sample-package
            name: sample-package-controller
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
              image: crossplane/sample-package:latest
              name: sample-package-controller
              resources: {}
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Secondcousin
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Cousin
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Sibling
  dependsOn:
  - crd: foo.mypackage.example.org/v1alpha1
  - crd: '*.yourpackage.example.org/v1alpha2'
  icons:
  - base64Data: bW9jay1pY29uLWRhdGE=
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  overview: text overview
  overviewShort: short text overview
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  packageType: Application
  permissionScope: Namespaced
  permissions:
    rules:
    - apiGroups:
      - ""
      resources:
      - configmaps
      - events
      - secrets
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - siblings
      - siblings/finalizers
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - secondcousins
      - secondcousins/status
      - secondcousins/scale
      - secondcousins/finalizers
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - mytypes
      - mytypes/finalizers
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - cousins
      - cousins/finalizers
      verbs:
      - '*'
    - apiGroups:
      - mypackage.example.org
      resources:
      - foo
      verbs:
      - '*'
    - apiGroups:
      - yourpackage.example.org
      resources:
      - '*'
      verbs:
      - '*'
  readme: |
    Markdown describing this sample Crossplane package project.
  source: https://github.com/crossplane/sample-package
  title: Sample Crossplane Package
  version: 0.0.1
  website: https://upbound.io
status:
  conditionedStatus: {}

---
`

	expectedComplexInfraPackageOutput = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/group-category: Group Category
    packages.crossplane.io/group-overview: Group Overview
    packages.crossplane.io/group-overview-short: Group Short Overview
    packages.crossplane.io/group-readme: Group Readme
    packages.crossplane.io/group-title: Group Title
    packages.crossplane.io/icon-data-uri: data:image/svg+xml;base64,bW9jay1pY29uLWRhdGEtc3Zn
    packages.crossplane.io/package-title: Sample Crossplane Package
    packages.crossplane.io/ui-schema: |-
      configSections:
      - title: group Title
        description: group Description
      ---
      configSections:
      - title: sibling Title
        description: sibling Description
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: environment
  name: siblings.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Sibling
    listKind: SiblingList
    plural: siblings
    singular: sibling
  scope: Namespaced
  version: v1alpha1
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    packages.crossplane.io/package-title: Sample Crossplane Package
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: environment
  name: secondcousins.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Secondcousin
    listKind: SecondcousinList
    plural: secondcousins
    singular: secondcousin
  scope: Namespaced
  subresources:
    scale:
      specReplicasPath: ""
      statusReplicasPath: ""
    status: {}
  version: v1alpha1
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/group-category: Group Category
    packages.crossplane.io/group-overview: Group Overview
    packages.crossplane.io/group-overview-short: Group Short Overview
    packages.crossplane.io/group-readme: Group Readme
    packages.crossplane.io/group-title: Group Title
    packages.crossplane.io/icon-data-uri: data:image/svg+xml;base64,c2luZ2xlLXJlc291cmNlLW1vY2staWNvbi1kYXRhLXN2Zw==
    packages.crossplane.io/package-title: Sample Crossplane Package
    packages.crossplane.io/resource-category: Resource Category
    packages.crossplane.io/resource-overview: Resource Overview
    packages.crossplane.io/resource-overview-short: Resource Short Overview
    packages.crossplane.io/resource-readme: Resource Readme
    packages.crossplane.io/resource-title: Resource Title
    packages.crossplane.io/resource-title-plural: Resources Title
    packages.crossplane.io/ui-schema: |-
      configSections:
      - title: group Title
        description: group Description
      ---
      configSections:
      - title: sibling Title
        description: sibling Description
      ---
      configSections:
      - title: kind Title
        description: kind Description
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: environment
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
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/group-category: Group Category
    packages.crossplane.io/group-overview: Group Overview
    packages.crossplane.io/group-overview-short: Group Short Overview
    packages.crossplane.io/group-readme: Group Readme
    packages.crossplane.io/group-title: Group Title
    packages.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    packages.crossplane.io/package-title: Sample Crossplane Package
    packages.crossplane.io/ui-schema: |-
      configSections:
      - title: group Title
        description: group Description
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: environment
  name: cousins.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: Cousin
    listKind: CousinList
    plural: cousins
    singular: cousin
  scope: Namespaced
  version: v1alpha1
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: packages.crossplane.io/v1alpha1
kind: Package
metadata:
  creationTimestamp: null
spec:
  category: Category
  company: Upbound
  controller:
    deployment:
      name: ""
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-package
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              core.crossplane.io/name: crossplane-sample-package
            name: sample-package-controller
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
              image: crossplane/sample-package:latest
              name: sample-package-controller
              resources: {}
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Secondcousin
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Cousin
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Sibling
  dependsOn:
  - crd: foo.mypackage.example.org/v1alpha1
  - crd: '*.yourpackage.example.org/v1alpha2'
  icons:
  - base64Data: bW9jay1pY29uLWRhdGE=
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  overview: text overview
  overviewShort: short text overview
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  packageType: Provider
  permissionScope: Cluster
  permissions:
    rules:
    - apiGroups:
      - ""
      resources:
      - configmaps
      - events
      - secrets
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - siblings
      - siblings/finalizers
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - secondcousins
      - secondcousins/status
      - secondcousins/scale
      - secondcousins/finalizers
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - mytypes
      - mytypes/finalizers
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - cousins
      - cousins/finalizers
      verbs:
      - '*'
    - apiGroups:
      - mypackage.example.org
      resources:
      - foo
      verbs:
      - '*'
    - apiGroups:
      - yourpackage.example.org
      resources:
      - '*'
      verbs:
      - '*'
  readme: |
    Markdown describing this sample Crossplane package project.
  source: https://github.com/crossplane/sample-package
  title: Sample Crossplane Package
  version: 0.0.1
  website: https://upbound.io
status:
  conditionedStatus: {}

---
`
)

var (
	// Assert on test that *PackagePackage implements PackagePackager
	_ PackagePackager = &PackagePackage{}
)

// simpleDeploymentInstallFile allows us to create an install file
// with different values, without having to have multiple copies
// of a whole install file
func simpleDeploymentInstallFile(image string) string {
	tmpl := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: crossplane-sample-package
  labels:
    core.crossplane.io/name: "crossplane-sample-package"
spec:
  selector:
    matchLabels:
      core.crossplane.io/name: "crossplane-sample-package"
  replicas: 1
  template:
    metadata:
      name: sample-package-controller
      labels:
        core.crossplane.io/name: "crossplane-sample-package"
    spec:
      containers:
      - name: sample-package-controller
        %s
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

	if image != "" {
		return fmt.Sprintf(tmpl, fmt.Sprintf("image: %s", image))

	}

	return fmt.Sprintf(tmpl, "")
}

func simpleBehaviorFile(sourceImage string) string {
	tmpl := `
crd:
  kind: SampleClaim
  apiVersion: samples.packages.crossplane.io/v1alpha1
engine:
  type: helm2
reconcile:
  path: 'resources'
source:
  %s
  path: /path
`

	if sourceImage != "" {
		return fmt.Sprintf(tmpl, fmt.Sprintf("image: %s", sourceImage))

	}

	return fmt.Sprintf(tmpl, "")
}

func simpleAppFile(permissionScope, packageType string, includeVersion bool) string {
	appFile := `# apiVersion this app.yaml conforms to
apiVersion: 0.1.0

# Human readable title of application.
title: Sample Crossplane Package

# Markdown description of this entry
readme: |
 Markdown describing this sample Crossplane package project.

overview: text overview
overviewShort: short text overview

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

# Category name.
category: Category

dependsOn:
- crd: "foo.mypackage.example.org/v1alpha1"
- crd: '*.yourpackage.example.org/v1alpha2'

# Keywords that describe this application and help search indexing
keywords:
- "samples"
- "examples"
- "tutorials"

# Links to more information about the application (about page, source code, etc.)
website: "https://upbound.io"
source: "https://github.com/crossplane/sample-package"
packageType: %q
permissionScope: %q

# License SPDX name: https://spdx.org/licenses/
license: Apache-2.0
`

	if includeVersion {
		appFile += `
# Version of project (optional)
# If omitted the version will be filled with the docker tag
# If set it must match the docker tag
version: 0.0.1
    `
	}

	return fmt.Sprintf(appFile, packageType, permissionScope)
}

func simpleCRDFile(singular string) string {
	title := strings.Title(singular)
	plural := singular + "s"
	return fmt.Sprintf(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: %s.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: %s
    listKind: %sList
    plural: %s
    singular: %s
  scope: Namespaced
  version: v1alpha1
`, plural, title, title, plural, singular)
}

func subresourceCRDFile(singular string) string {
	title := strings.Title(singular)
	plural := singular + "s"
	return fmt.Sprintf(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: %s.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
    kind: %s
    listKind: %sList
    plural: %s
    singular: %s
  scope: Namespaced
  subresources:
    status: {}
    scale: {}
  version: v1alpha1
`, plural, title, title, plural, singular)
}

func simpleUIFile(name string) string {
	return fmt.Sprintf(`configSections:
- title: %s Title
  description: %s Description
`, name, name)
}

func expectedSimpleDeploymentPackageOutput(controllerImage string) string {
	tmpl := `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    packages.crossplane.io/package-title: Sample Crossplane Package
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: namespace
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
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: packages.crossplane.io/v1alpha1
kind: Package
metadata:
  creationTimestamp: null
spec:
  category: Category
  company: Upbound
  controller:
    deployment:
      name: ""
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-package
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              core.crossplane.io/name: crossplane-sample-package
            name: sample-package-controller
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
              %sname: sample-package-controller
              resources: {}
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  dependsOn:
  - crd: foo.mypackage.example.org/v1alpha1
  - crd: '*.yourpackage.example.org/v1alpha2'
  icons:
  - base64Data: bW9jay1pY29uLWRhdGE=
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  overview: text overview
  overviewShort: short text overview
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  packageType: Application
  permissionScope: Namespaced
  permissions:
    rules:
    - apiGroups:
      - ""
      resources:
      - configmaps
      - events
      - secrets
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - mytypes
      - mytypes/finalizers
      verbs:
      - '*'
    - apiGroups:
      - mypackage.example.org
      resources:
      - foo
      verbs:
      - '*'
    - apiGroups:
      - yourpackage.example.org
      resources:
      - '*'
      verbs:
      - '*'
  readme: |
    Markdown describing this sample Crossplane package project.
  source: https://github.com/crossplane/sample-package
  title: Sample Crossplane Package
  version: 0.0.1
  website: https://upbound.io
status:
  conditionedStatus: {}

---
`

	if controllerImage != "" {
		// The spaces are used for formatting the next line. This is a quick and dirty way
		// to optionally insert an additional line into the output.
		return fmt.Sprintf(tmpl, fmt.Sprintf("image: %s\n              ", controllerImage))
	}

	return fmt.Sprintf(tmpl, "")
}

func expectedSimpleBehaviorPackageOutput(sourceImage string) string {
	tmpl := `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    packages.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    packages.crossplane.io/package-title: Sample Crossplane Package
  creationTimestamp: null
  labels:
    app.kubernetes.io/managed-by: package-manager
    crossplane.io/scope: namespace
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
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null

---
apiVersion: packages.crossplane.io/v1alpha1
kind: StackDefinition
metadata:
  creationTimestamp: null
spec:
  behavior:
    crd:
      apiVersion: samples.packages.crossplane.io/v1alpha1
      kind: SampleClaim
    engine:
      controllerImage: crossplane/ts-controller:0.0.0
      type: helm2
    source:
      %spath: /path
  category: Category
  company: Upbound
  controller:
    deployment:
      name: ""
      spec:
        selector: {}
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
          spec:
            containers:
            - args:
              - --resources-dir
              - /behaviors
              - --stack-definition-namespace
              - $(SD_NAMESPACE)
              - --stack-definition-name
              - $(SD_NAME)
              image: crossplane/ts-controller:0.0.0
              name: package-behavior-manager
              resources: {}
              volumeMounts:
              - mountPath: /behaviors
                name: behaviors
            initContainers:
            - command:
              - cp
              - -R
              - /path/.
              - /behaviors
              %sname: package-behavior-copy-to-manager
              resources: {}
              volumeMounts:
              - mountPath: /behaviors
                name: behaviors
            restartPolicy: Always
            volumes:
            - emptyDir: {}
              name: behaviors
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  dependsOn:
  - crd: foo.mypackage.example.org/v1alpha1
  - crd: '*.yourpackage.example.org/v1alpha2'
  icons:
  - base64Data: bW9jay1pY29uLWRhdGE=
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  overview: text overview
  overviewShort: short text overview
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  packageType: Application
  permissionScope: Namespaced
  permissions:
    rules:
    - apiGroups:
      - ""
      resources:
      - configmaps
      - events
      - secrets
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - mytypes
      - mytypes/finalizers
      verbs:
      - '*'
    - apiGroups:
      - mypackage.example.org
      resources:
      - foo
      verbs:
      - '*'
    - apiGroups:
      - yourpackage.example.org
      resources:
      - '*'
      verbs:
      - '*'
  readme: |
    Markdown describing this sample Crossplane package project.
  source: https://github.com/crossplane/sample-package
  title: Sample Crossplane Package
  version: 0.0.1
  website: https://upbound.io
status: {}

---
`

	if sourceImage != "" {
		// The spaces are used for formatting the next line. This is a quick and dirty way
		// to optionally insert an additional line into the output.
		return fmt.Sprintf(tmpl, fmt.Sprintf("image: %s\n      ", sourceImage), fmt.Sprintf("image: %s\n              ", sourceImage))
	}

	return fmt.Sprintf(tmpl, "", "")
}

func TestUnpack(t *testing.T) {
	type want struct {
		output string
		err    error
	}

	tests := []struct {
		name         string
		packageImage string
		fs           afero.Fs
		root         string
		want         want
	}{
		{
			// unpack should fail to find the install.yaml file
			name: "EmptyPackageDir",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: "", err: errors.New("Package does not contain an app.yaml file")},
		},
		{
			name: "SimpleDeploymentPackage",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced", "Application", true)), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile("crossplane/sample-package:latest")), 0644)
				crdDir := simpleCrdDir
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedSimpleDeploymentPackageOutput("crossplane/sample-package:latest"), err: nil},
		},
		{
			name: "SimpleDeploymentPackageWithNoVersionShouldHaveNoVersion",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced", "Application", true)), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile("")), 0644)
				crdDir := simpleCrdDir
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedSimpleDeploymentPackageOutput(""), err: nil},
		},
		{
			name: "ReadVersionFromPackageImage",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced", "Application", false)), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile("crossplane/sample-package:latest")), 0644)
				crdDir := simpleCrdDir
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				return fs
			}(),
			root:         "ext-dir",
			want:         want{output: expectedSimpleDeploymentPackageOutput("crossplane/sample-package:latest"), err: nil},
			packageImage: "crossplane/sample-package:0.0.1",
		},
		{
			name: "SimpleBehaviorPackage",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced", "Application", true)), 0644)
				afero.WriteFile(fs, "ext-dir/behavior.yaml", []byte(simpleBehaviorFile("crossplane/sample-package-claim-test:helm2")), 0644)
				crdDir := simpleCrdDir
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedSimpleBehaviorPackageOutput("crossplane/sample-package-claim-test:helm2"), err: nil},
		},
		{
			name: "SimpleBehaviorPackageWithNoVersionShouldHaveNoVersion",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced", "Application", true)), 0644)
				afero.WriteFile(fs, "ext-dir/behavior.yaml", []byte(simpleBehaviorFile("")), 0644)
				crdDir := simpleCrdDir
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedSimpleBehaviorPackageOutput(""), err: nil},
		},
		{
			name: "ComplexDeploymentPackage",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				groupDir := "ext-dir/resources/samples.upbound.io"
				groupDir2 := "ext-dir/resources/other.upbound.io"

				// secondcousins share root path resources
				// cousins share that and group path resources
				// siblings share that and crd path resources

				crdDir := filepath.Join(groupDir, "mytype/v1alpha1")
				crdDir2 := filepath.Join(groupDir, "cousin/v1alpha1")
				crdDir3 := filepath.Join(groupDir2, "secondcousin/v1alpha1")

				for _, d := range []string{crdDir, crdDir2, crdDir3} {
					fs.MkdirAll(d, 0755)
				}

				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced", "Application", true)), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile("crossplane/sample-package:latest")), 0644)
				afero.WriteFile(fs, filepath.Join(groupDir, "group.yaml"), []byte(simpleGroupFile), 0644)
				afero.WriteFile(fs, filepath.Join(groupDir, "ui-schema.yaml"), []byte(simpleUIFile("group")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "icon.png"), []byte("mock-icon-data-png"), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "icon.svg"), []byte("mock-icon-data-svg"), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "resource.yaml"), []byte(simpleResourceFile), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "ui-schema.yaml"), []byte(simpleUIFile("sibling")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.ui-schema.yaml"), []byte(simpleUIFile("kind")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "unmatched.ui-schema.yaml"), []byte(simpleUIFile("mismatch")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "sibling.v1alpha1.crd.yaml"), []byte(simpleCRDFile("sibling")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir2, "cousin.v1alpha1.crd.yaml"), []byte(simpleCRDFile("cousin")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir3, "secondcousin.v1alpha1.crd.yaml"), []byte(subresourceCRDFile("secondcousin")), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedComplexDeploymentPackageOutput, err: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &bytes.Buffer{}
			rd := &walker.ResourceDir{Base: tt.root, Walker: afero.Afero{Fs: tt.fs}}

			os.Setenv(PackageImageEnv, tt.packageImage)
			err := Unpack(rd, got, tt.root, "Namespaced", "crossplane/ts-controller:0.0.0", logging.NewLogrLogger(zap.Logger(true)))
			os.Unsetenv(PackageImageEnv)

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Unpack() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.output, got.String()); diff != "" {
				t.Errorf("Unpack() -want, +got:\n%v", diff)
			}
		})
	}
}

func TestUnpackCluster(t *testing.T) {
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
			name: "ComplexInfraPackage",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				groupDir := "ext-dir/resources/samples.upbound.io"
				groupDir2 := "ext-dir/resources/other.upbound.io"

				// secondcousins share root path resources
				// cousins share that and group path resources
				// siblings share that and crd path resources

				crdDir := filepath.Join(groupDir, "mytype/v1alpha1")
				crdDir2 := filepath.Join(groupDir, "cousin/v1alpha1")
				crdDir3 := filepath.Join(groupDir2, "secondcousin/v1alpha1")

				for _, d := range []string{crdDir, crdDir2, crdDir3} {
					fs.MkdirAll(d, 0755)
				}

				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Cluster", "Provider", true)), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile("crossplane/sample-package:latest")), 0644)
				afero.WriteFile(fs, filepath.Join(groupDir, "group.yaml"), []byte(simpleGroupFile), 0644)
				afero.WriteFile(fs, filepath.Join(groupDir, "ui-schema.yaml"), []byte(simpleUIFile("group")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "icon.png"), []byte("mock-icon-data-png"), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "icon.svg"), []byte("mock-icon-data-svg"), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.icon.svg"), []byte("single-resource-mock-icon-data-svg"), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "resource.yaml"), []byte(simpleResourceFile), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "ui-schema.yaml"), []byte(simpleUIFile("sibling")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.ui-schema.yaml"), []byte(simpleUIFile("kind")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "unmatched.ui-schema.yaml"), []byte(simpleUIFile("mismatch")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir, "sibling.v1alpha1.crd.yaml"), []byte(simpleCRDFile("sibling")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir2, "cousin.v1alpha1.crd.yaml"), []byte(simpleCRDFile("cousin")), 0644)
				afero.WriteFile(fs, filepath.Join(crdDir3, "secondcousin.v1alpha1.crd.yaml"), []byte(subresourceCRDFile("secondcousin")), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedComplexInfraPackageOutput, err: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &bytes.Buffer{}
			rd := &walker.ResourceDir{Base: tt.root, Walker: afero.Afero{Fs: tt.fs}}
			err := Unpack(rd, got, tt.root, "Cluster", "crossplane/ts-controller:0.0.0", logging.NewLogrLogger(zap.Logger(true)))

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Unpack() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.output, got.String()); diff != "" {
				t.Errorf("Unpack() -want, +got:\n%v", diff)
			}
		})
	}
}

func TestOrderPackageIconKeys(t *testing.T) {
	type args struct {
		m map[string]*v1alpha1.IconSpec
	}

	tests := []struct {
		name string
		args args
		want []string
	}{{"empty",
		args{map[string]*v1alpha1.IconSpec{}},
		[]string{},
	}, {"basic",
		args{map[string]*v1alpha1.IconSpec{"a": nil}},
		[]string{"a"},
	}, {"full",
		args{map[string]*v1alpha1.IconSpec{"/": nil, "/foo/bar": nil, "/bar": nil, "/foo": nil}},
		[]string{"/foo/bar", "/foo", "/bar", "/"},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orderPackageIconKeys(tt.args.m)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("orderPackageIconKeys(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsMetadataApplicableToCRD(t *testing.T) {
	type args struct {
		crdPath         string
		metadataPath    string
		globalFileNames []string
		crdKind         string
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "GlobalFileRootMatch",
			args: args{
				crdPath:         "/a/b/c/crd.yaml",
				metadataPath:    "/icon.svg",
				globalFileNames: iconFileGlobalNames,
				crdKind:         "mytype",
			},
			want: true,
		},
		{
			name: "GlobalFileParentMatch",
			args: args{
				crdPath:         "/a/b/c/crd.yaml",
				metadataPath:    "/a/b/ui-schema.yaml",
				globalFileNames: uiSchemaFileGlobalNames,
				crdKind:         "mytype",
			},
			want: true,
		},
		{
			name: "GlobalFileParentSingleResourceFileMatch",
			args: args{
				crdPath:         "/a/b/c/mytype.crd.yaml",
				metadataPath:    "/a/b/ui-schema.yaml",
				globalFileNames: uiSchemaFileGlobalNames,
				crdKind:         "mytype",
			},
			want: true,
		},
		{
			name: "GlobalFileSiblingMatch",
			args: args{
				crdPath:         "/a/b/c/crd.yaml",
				metadataPath:    "/a/b/c/icon.svg",
				globalFileNames: iconFileGlobalNames,
				crdKind:         "mytype",
			},
			want: true,
		},
		{
			name: "GlobalFileCousinNoMatch",
			args: args{
				crdPath:         "/a/b/c/crd.yaml",
				metadataPath:    "/z/icon.svg",
				globalFileNames: iconFileGlobalNames,
				crdKind:         "mytype",
			},
			want: false,
		},
		{
			name: "SingleResourceFileKindMatch",
			args: args{
				crdPath:         "/a/b/c/crd.yaml",
				metadataPath:    "/a/b/c/mytype.icon.svg",
				globalFileNames: iconFileGlobalNames,
				crdKind:         "mytype",
			},
			want: true,
		},
		{
			name: "SingleResourceFileKindNoMatch",
			args: args{
				crdPath:         "/a/b/c/crd.yaml",
				metadataPath:    "/a/b/c/mytype.icon.svg",
				globalFileNames: iconFileGlobalNames,
				crdKind:         "yourtype",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMetadataApplicableToCRD(tt.args.crdPath, tt.args.metadataPath, tt.args.globalFileNames, tt.args.crdKind)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("%s isMetadataApplicableToCRD(): -want, +got:\n%s", tt.name, diff)
			}
		})
	}
}
