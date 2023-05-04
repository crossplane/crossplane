Feature: Use Composite Resources to build own APIs

  As a user of Crossplane
  I want to Use Composite Resources to build my own platform with opinionated concepts and APIs
  without needing to write a Kubernetes controller from scratch.

  Background:
    Given Crossplane is running in cluster

  Scenario: Create managed resources through claim creation
    Given provider xpkg.upbound.io/upbound/provider-dummy:v0.3.0 is running in cluster
    And CompositeResourceDefinition is present
      """
        apiVersion: apiextensions.crossplane.io/v1
        kind: CompositeResourceDefinition
        metadata:
          name: xrobots.dummy.crossplane.io
          labels:
            provider: dummy-provider
        spec:
          defaultCompositionRef:
            name: robots-test
          group: dummy.crossplane.io
          names:
            kind: XRobot
            plural: xrobots
          claimNames:
            kind: Robot
            plural: robots
          versions:
            - name: v1alpha1
              served: true
              referenceable: true
              schema:
                openAPIV3Schema:
                  type: object
                  properties:
                    spec:
                      type: object
                      properties:
                        color:
                          type: string
                      required:
                        - color
      """
    And Composition is present
      """
        apiVersion: apiextensions.crossplane.io/v1
        kind: Composition
        metadata:
          name: robots-test
          labels:
            crossplane.io/xrd: xrobots.dummy.crossplane.io
            provider: dummy-provider
        spec:
          compositeTypeRef:
            apiVersion: dummy.crossplane.io/v1alpha1
            kind: XRobot
          writeConnectionSecretsToNamespace: default
          resources:
            - name: robot
              base:
                apiVersion: iam.dummy.upbound.io/v1alpha1
                kind: Robot
                spec:
                  forProvider: {}
              patches:
                - type: FromCompositeFieldPath
                  fromFieldPath: spec.color
                  toFieldPath: spec.forProvider.color
      """
    When claim gets deployed
      """
        apiVersion: dummy.crossplane.io/v1alpha1
        kind: Robot
        metadata:
          name: test-robot
        spec:
          color: blue
      """
    Then claim becomes synchronized and ready
    And claim composite resource becomes synchronized and ready
    And composed managed resources become ready and synchronized

  Scenario: Create provider-nop managed resource through claim
    Given Crossplane is running in cluster
    And provider crossplane/provider-nop:main is running in cluster
    And CompositeResourceDefinition is present
      """
        apiVersion: apiextensions.crossplane.io/v1
        kind: CompositeResourceDefinition
        metadata:
          name: clusternopresources.nop.example.org
        spec:
          group: nop.example.org
          names:
            kind: ClusterNopResource
            listKind: ClusterNopResourceList
            plural: clusternopresources
            singular: clusternopresource
          claimNames:
            kind: NopResource
            listKind: NopResourceList
            plural: nopresources
            singular: nopresource
          connectionSecretKeys:
            - test
          versions:
            - name: v1alpha1
              served: true
              referenceable: true
              schema:
                openAPIV3Schema:
                  type: object
                  properties:
                    spec:
                      type: object
                      properties:
                        coolField:
                          type: string
                      required:
                        - coolField
      """
    And Composition is present
      """
        apiVersion: apiextensions.crossplane.io/v1
        kind: Composition
        metadata:
          name: clusternopresources.sample.nop.example.org
          labels:
            provider: provider-nop
        spec:
          compositeTypeRef:
            apiVersion: nop.example.org/v1alpha1
            kind: ClusterNopResource
          resources:
            - name: nopinstance1
              base:
                apiVersion: nop.crossplane.io/v1alpha1
                kind: NopResource
                spec:
                  forProvider:
                    conditionAfter:
                      - conditionType: Ready
                        conditionStatus: "False"
                        time: 0s
                      - conditionType: Ready
                        conditionStatus: "True"
                        time: 10s
                      - conditionType: Synced
                        conditionStatus: "False"
                        time: 0s
                      - conditionType: Synced
                        conditionStatus: "True"
                        time: 10s
                  writeConnectionSecretsToRef:
                    namespace: crossplane-system
                    name: nop-example-resource
            - name: nopinstance2
              base:
                apiVersion: nop.crossplane.io/v1alpha1
                kind: NopResource
                spec:
                  forProvider:
                    conditionAfter:
                      - conditionType: Ready
                        conditionStatus: "False"
                        time: 0s
                      - conditionType: Ready
                        conditionStatus: "True"
                        time: 10s
                      - conditionType: Synced
                        conditionStatus: "False"
                        time: 0s
                      - conditionType: Synced
                        conditionStatus: "True"
                        time: 10s
                  writeConnectionSecretsToRef:
                    namespace: crossplane-system
                    name: nop-example-resource
      """
    When claim gets deployed
      """
        apiVersion: nop.example.org/v1alpha1
        kind: NopResource
        metadata:
          name: nop-example
        spec:
          coolField: example
      """
    Then claim becomes synchronized and ready
    And claim composite resource becomes synchronized and ready
    And composed managed resources become ready and synchronized