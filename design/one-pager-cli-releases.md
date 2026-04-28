# Decoupling CLI Releases from Crossplane Core Releases

* Owner: Adam Wolfe Gordon (@adamwg)
* Reviewers: Jared Watts (@jbw976), Nic Cope (@negz)
* Status: Proposed

## Background

As part of the project to add developer experience tooling to the Crossplane
CLI, we have decided to move the CLI to its own repository,
`crossplane/cli`. This move has a few motivations:

1. Decouple the feature development and release cadence of the CLI from
   Crossplane core, with the intent that CLI development can move faster.
2. Make it simpler for the CLI to have its own set of maintainers, who are not
   necessarily all core Crossplane maintainers.
3. Enforce architectural boundaries between the CLI and Crossplane core (since
   the CLI can no longer import `internal/` packages from core).

The first motivation being the primary and most compelling one, we must now
define how CLI releases will relate to Crossplane core releases and by what
mechanism they will be made available for installation.

## Proposal

### Versioning

I propose that the initial CLI release from its new repository coincide with the
v2.3.0 release of Crossplane, and carry the version number v2.3.0. This makes it
clear that CLI development has continued in a new location and that users should
update from v2.2.x or older releases.

I propose that after the initial release, we allow the CLI's version to diverge
from that of Crossplane core. In keeping with semantic versioning, the next
bugfix release of the CLI following v2.3.0 will be v2.3.1, and the next release
containing new features will be v2.4.0, regardless of whether its release date
coincides with Crossplane v2.4.0.

### Compatibility

Given that the Crossplane CLI version number will no longer correspond to the
Crossplane core version, we must define a policy for backward compatibility.

In practice, I expect this will be largely a non-issue: going forward, all
interactions between the CLI and Crossplane core will be across API boundaries,
and Crossplane's APIs are relatively stable. We will, of course, introduce new
CLI features that require new APIs and work only on newer versions of
Crossplane, but existing features are unlikely to break.

That being said, we still need a policy. I propose that:

1. Pre-existing features in a new minor version of the Crossplane CLI will
   maintain support for all Crossplane minor versions that are supported at
   release time. For example, if we released CLI v2.4.0 in June, 2026 we would
   support Crossplane v2.3, v2.2, and v2.1.
2. Patch releases of the CLI will not break compatibility with any Crossplane
   releases that were supported when their minor version was initially released,
   even if it has since become EOL. I.e., changes to the compatibility matrix
   will happen only in new minor versions of the CLI.

### Artifact Location

I propose that the CLI, going forward, will be distributed via a new S3 bucket
called `crossplane-cli-releases` (separate from the existing
`crossplane-releases` bucket used for Crossplane core artifacts). This new
bucket will have the same layout as `crossplane-releases`, but contain only the
CLI binaries and bundles.

We do not currently distribute OCI images for the CLI, and will not start doing
so immediately. If we do, they will be uploaded to the release bucket as
tarballs and also pushed to GHCR, as the Crossplane core images are today.

## Necessary Changes

When we move the CLI into `crossplane/cli`, that repository will be bootstrapped
with its own Nix-based build infrastructure configured to upload to the
`crossplane-cli-releases` bucket. We should set up a new CloudFront
distribution, `cli.crossplane.io`, to serve CLI releases as
`releases.crossplane.io` does for Crossplane core.

The `install.sh` script can be uploaded to the new bucket, so that users are
able to install the CLI by running:

```console
curl -sL https://cli.crossplane.io | sh
```

The install script will need to be updated to reflect the new bucket
location. For the time being, the install script can continue to also be
available in the Crossplane core repository, but updated to point at the new
bucket, so that any scripts relying on the current instructions (getting the
script directly from the repository via `raw.githubusercontent.com`) continue to
work.
