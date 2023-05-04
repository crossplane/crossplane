Feature: Use configuration package to install all needed providers and other needed resources

  As a user of Crossplane, I would like to be able to manage all required providers and other resources
  by defining a configuration that refers all of them, so that they could be installed/managed/removed as a group

  Background:
    Given Crossplane is running in cluster

  Scenario: install provider by declaring it as a part of a configuration
    When Configuration is applied
      """
        apiVersion: pkg.crossplane.io/v1
        kind: Configuration
        metadata:
          name: my-org-infra
        spec:
          package: xpkg.upbound.io/xp/getting-started-with-aws:v1.12.1
      """
    Then configuration is marked as installed and healthy
    And provider crossplane-contrib-provider-aws is marked as installed and healthy

  Scenario: install configuration without installing its dependencies
    When Configuration is applied
      """
        apiVersion: pkg.crossplane.io/v1
        kind: Configuration
        metadata:
          name: my-org-infra-no-deps
        spec:
          package: xpkg.upbound.io/devops-toolkit/dot-gitops:v0.2.14
          skipDependencyResolution: true
      """
    Then configuration is marked as installed and healthy
    And provider crossplane-provider-helm does not get installed
