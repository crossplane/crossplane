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

package stacks

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/stacks/walker"
)

const (
	simpleDeploymentInstallFile = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: crossplane-sample-stack
  labels:
    core.crossplane.io/name: "crossplane-sample-stack"
spec:
  selector:
    matchLabels:
      core.crossplane.io/name: "crossplane-sample-stack"
  replicas: 1
  template:
    metadata:
      name: sample-stack-controller
      labels:
        core.crossplane.io/name: "crossplane-sample-stack"
    spec:
      containers:
      - name: sample-stack-controller
        image: crossplane/sample-stack:latest
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
      - name: sample-stack-from-job
        image: crossplane/sample-stack-from-job:latest
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

	expectedSimpleDeploymentStackOutput = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
  creationTimestamp: null
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
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  creationTimestamp: null
spec:
  category: Category
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-stack
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-stack
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              core.crossplane.io/name: crossplane-sample-stack
            name: sample-stack-controller
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
              image: crossplane/sample-stack:latest
              name: sample-stack-controller
              resources: {}
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  dependsOn:
  - crd: foo.mystack.example.org/v1alpha1
  - crd: '*.yourstack.example.org/v1alpha2'
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
      verbs:
      - '*'
    - apiGroups:
      - mystack.example.org
      resources:
      - foo
      verbs:
      - '*'
    - apiGroups:
      - yourstack.example.org
      resources:
      - '*'
      verbs:
      - '*'
  readme: |
    Markdown describing this sample Crossplane stack project.
  source: https://github.com/crossplaneio/sample-stack
  title: Sample Crossplane Stack
  version: 0.0.1
  website: https://upbound.io
status:
  conditionedStatus: {}
`

	expectedComplexDeploymentStackOutput = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/group-category: Group Category
    stacks.crossplane.io/group-overview: Group Overview
    stacks.crossplane.io/group-overview-short: Group Short Overview
    stacks.crossplane.io/group-readme: Group Readme
    stacks.crossplane.io/group-title: Group Title
    stacks.crossplane.io/icon-data-uri: data:image/svg+xml;base64,bW9jay1pY29uLWRhdGEtc3Zn
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
    stacks.crossplane.io/ui-spec: |-
      uiSpecVersion: 0.3
      uiSpec:
      - title: group Title
        description: group Description
      ---
      uiSpecVersion: 0.3
      uiSpec:
      - title: sibling Title
        description: sibling Description
  creationTimestamp: null
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
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
  creationTimestamp: null
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
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/group-category: Group Category
    stacks.crossplane.io/group-overview: Group Overview
    stacks.crossplane.io/group-overview-short: Group Short Overview
    stacks.crossplane.io/group-readme: Group Readme
    stacks.crossplane.io/group-title: Group Title
    stacks.crossplane.io/icon-data-uri: data:image/svg+xml;base64,bW9jay1pY29uLWRhdGEtc3Zn
    stacks.crossplane.io/resource-category: Resource Category
    stacks.crossplane.io/resource-overview: Resource Overview
    stacks.crossplane.io/resource-overview-short: Resource Short Overview
    stacks.crossplane.io/resource-readme: Resource Readme
    stacks.crossplane.io/resource-title: Resource Title
    stacks.crossplane.io/resource-title-plural: Resources Title
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
    stacks.crossplane.io/ui-spec: |-
      uiSpecVersion: 0.3
      uiSpec:
      - title: group Title
        description: group Description
      ---
      uiSpecVersion: 0.3
      uiSpec:
      - title: sibling Title
        description: sibling Description
      ---
      uiSpecVersion: 0.3
      uiSpec:
      - title: kind Title
        description: kind Description
  creationTimestamp: null
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
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/group-category: Group Category
    stacks.crossplane.io/group-overview: Group Overview
    stacks.crossplane.io/group-overview-short: Group Short Overview
    stacks.crossplane.io/group-readme: Group Readme
    stacks.crossplane.io/group-title: Group Title
    stacks.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
    stacks.crossplane.io/ui-spec: |-
      uiSpecVersion: 0.3
      uiSpec:
      - title: group Title
        description: group Description
  creationTimestamp: null
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
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  creationTimestamp: null
spec:
  category: Category
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-stack
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-stack
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              core.crossplane.io/name: crossplane-sample-stack
            name: sample-stack-controller
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
              image: crossplane/sample-stack:latest
              name: sample-stack-controller
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
  - crd: foo.mystack.example.org/v1alpha1
  - crd: '*.yourstack.example.org/v1alpha2'
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
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - secondcousins
      - secondcousins/status
      - secondcousins/scale
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - mytypes
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - cousins
      verbs:
      - '*'
    - apiGroups:
      - mystack.example.org
      resources:
      - foo
      verbs:
      - '*'
    - apiGroups:
      - yourstack.example.org
      resources:
      - '*'
      verbs:
      - '*'
  readme: |
    Markdown describing this sample Crossplane stack project.
  source: https://github.com/crossplaneio/sample-stack
  title: Sample Crossplane Stack
  version: 0.0.1
  website: https://upbound.io
status:
  conditionedStatus: {}
`

	expectedComplexInfraStackOutput = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/group-category: Group Category
    stacks.crossplane.io/group-overview: Group Overview
    stacks.crossplane.io/group-overview-short: Group Short Overview
    stacks.crossplane.io/group-readme: Group Readme
    stacks.crossplane.io/group-title: Group Title
    stacks.crossplane.io/icon-data-uri: data:image/svg+xml;base64,bW9jay1pY29uLWRhdGEtc3Zn
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
    stacks.crossplane.io/ui-spec: |-
      uiSpecVersion: 0.3
      uiSpec:
      - title: group Title
        description: group Description
      ---
      uiSpecVersion: 0.3
      uiSpec:
      - title: sibling Title
        description: sibling Description
  creationTimestamp: null
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
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
  creationTimestamp: null
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
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/group-category: Group Category
    stacks.crossplane.io/group-overview: Group Overview
    stacks.crossplane.io/group-overview-short: Group Short Overview
    stacks.crossplane.io/group-readme: Group Readme
    stacks.crossplane.io/group-title: Group Title
    stacks.crossplane.io/icon-data-uri: data:image/svg+xml;base64,c2luZ2xlLXJlc291cmNlLW1vY2staWNvbi1kYXRhLXN2Zw==
    stacks.crossplane.io/resource-category: Resource Category
    stacks.crossplane.io/resource-overview: Resource Overview
    stacks.crossplane.io/resource-overview-short: Resource Short Overview
    stacks.crossplane.io/resource-readme: Resource Readme
    stacks.crossplane.io/resource-title: Resource Title
    stacks.crossplane.io/resource-title-plural: Resources Title
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
    stacks.crossplane.io/ui-spec: |-
      uiSpecVersion: 0.3
      uiSpec:
      - title: group Title
        description: group Description
      ---
      uiSpecVersion: 0.3
      uiSpec:
      - title: sibling Title
        description: sibling Description
      ---
      uiSpecVersion: 0.3
      uiSpec:
      - title: kind Title
        description: kind Description
  creationTimestamp: null
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
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/group-category: Group Category
    stacks.crossplane.io/group-overview: Group Overview
    stacks.crossplane.io/group-overview-short: Group Short Overview
    stacks.crossplane.io/group-readme: Group Readme
    stacks.crossplane.io/group-title: Group Title
    stacks.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
    stacks.crossplane.io/ui-spec: |-
      uiSpecVersion: 0.3
      uiSpec:
      - title: group Title
        description: group Description
  creationTimestamp: null
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
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  creationTimestamp: null
spec:
  category: Category
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-stack
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-stack
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              core.crossplane.io/name: crossplane-sample-stack
            name: sample-stack-controller
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
              image: crossplane/sample-stack:latest
              name: sample-stack-controller
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
  - crd: foo.mystack.example.org/v1alpha1
  - crd: '*.yourstack.example.org/v1alpha2'
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
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - secondcousins
      - secondcousins/status
      - secondcousins/scale
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - mytypes
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io
      resources:
      - cousins
      verbs:
      - '*'
    - apiGroups:
      - mystack.example.org
      resources:
      - foo
      verbs:
      - '*'
    - apiGroups:
      - yourstack.example.org
      resources:
      - '*'
      verbs:
      - '*'
  readme: |
    Markdown describing this sample Crossplane stack project.
  source: https://github.com/crossplaneio/sample-stack
  title: Sample Crossplane Stack
  version: 0.0.1
  website: https://upbound.io
status:
  conditionedStatus: {}
`

	expectedSimpleJobStackOutput = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    app.kubernetes.io/managed-by: stack-manager
    stacks.crossplane.io/icon-data-uri: data:image/jpeg;base64,bW9jay1pY29uLWRhdGE=
    stacks.crossplane.io/stack-title: Sample Crossplane Stack
  creationTimestamp: null
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
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  creationTimestamp: null
spec:
  category: Category
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
              image: crossplane/sample-stack-from-job:latest
              name: sample-stack-from-job
              resources: {}
            restartPolicy: Never
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  dependsOn:
  - crd: foo.mystack.example.org/v1alpha1
  - crd: '*.yourstack.example.org/v1alpha2'
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
      verbs:
      - '*'
    - apiGroups:
      - mystack.example.org
      resources:
      - foo
      verbs:
      - '*'
    - apiGroups:
      - yourstack.example.org
      resources:
      - '*'
      verbs:
      - '*'
  readme: |
    Markdown describing this sample Crossplane stack project.
  source: https://github.com/crossplaneio/sample-stack
  title: Sample Crossplane Stack
  version: 0.0.1
  website: https://upbound.io
status:
  conditionedStatus: {}
`
)

var (
	// Assert on test that *StackPackage implements StackPackager
	_ StackPackager = &StackPackage{}
)

func simpleAppFile(permissionScope string) string {
	return fmt.Sprintf(`# Human readable title of application.
title: Sample Crossplane Stack

# Markdown description of this entry
readme: |
 Markdown describing this sample Crossplane stack project.

overview: text overview
overviewShort: short text overview

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

# Category name.
category: Category

dependsOn:
- crd: "foo.mystack.example.org/v1alpha1"
- crd: '*.yourstack.example.org/v1alpha2'

# Keywords that describe this application and help search indexing
keywords:
- "samples"
- "examples"
- "tutorials"

# Links to more information about the application (about page, source code, etc.)
website: "https://upbound.io"
source: "https://github.com/crossplaneio/sample-stack"

permissionScope: %q

# License SPDX name: https://spdx.org/licenses/
license: Apache-2.0
`, permissionScope)
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
	return fmt.Sprintf(`uiSpecVersion: 0.3
uiSpec:
- title: %s Title
  description: %s Description
`, name, name)
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
			name: "EmptyStackDir",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: "", err: errors.New("Stack does not contain an app.yaml file")},
		},
		{
			name: "SimpleDeploymentStack",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced")), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile), 0644)
				crdDir := "ext-dir/resources/samples.upbound.io/mytype/v1alpha1"
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedSimpleDeploymentStackOutput, err: nil},
		},
		{
			name: "ComplexDeploymentStack",
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
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced")), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile), 0644)
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
			want: want{output: expectedComplexDeploymentStackOutput, err: nil},
		},
		{
			name: "SimpleJobStack",
			fs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				fs.MkdirAll("ext-dir", 0755)
				afero.WriteFile(fs, "ext-dir/icon.jpg", []byte("mock-icon-data"), 0644)
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Namespaced")), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleJobInstallFile), 0644)
				crdDir := "ext-dir/resources/samples.upbound.io/mytype/v1alpha1"
				fs.MkdirAll(crdDir, 0755)
				afero.WriteFile(fs, filepath.Join(crdDir, "mytype.v1alpha1.crd.yaml"), []byte(simpleCRDFile("mytype")), 0644)
				return fs
			}(),
			root: "ext-dir",
			want: want{output: expectedSimpleJobStackOutput, err: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &bytes.Buffer{}
			rd := &walker.ResourceDir{Base: tt.root, Walker: afero.Afero{Fs: tt.fs}}
			err := Unpack(rd, got, tt.root, "Namespaced")

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
			name: "ComplexInfraStack",
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
				afero.WriteFile(fs, "ext-dir/app.yaml", []byte(simpleAppFile("Cluster")), 0644)
				afero.WriteFile(fs, "ext-dir/install.yaml", []byte(simpleDeploymentInstallFile), 0644)
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
			want: want{output: expectedComplexInfraStackOutput, err: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &bytes.Buffer{}
			rd := &walker.ResourceDir{Base: tt.root, Walker: afero.Afero{Fs: tt.fs}}
			err := Unpack(rd, got, tt.root, "Cluster")

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Unpack() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.output, got.String()); diff != "" {
				t.Errorf("Unpack() -want, +got:\n%v", diff)
			}
		})
	}
}

func TestOrderStackIconKeys(t *testing.T) {
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
			got := orderStackIconKeys(tt.args.m)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("orderStackIconKeys(): -want, +got:\n%s", diff)
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
				globalFileNames: uiSpecFileGlobalNames,
				crdKind:         "mytype",
			},
			want: true,
		},
		{
			name: "GlobalFileParentSingleResourceFileMatch",
			args: args{
				crdPath:         "/a/b/c/mytype.crd.yaml",
				metadataPath:    "/a/b/ui-schema.yaml",
				globalFileNames: uiSpecFileGlobalNames,
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
