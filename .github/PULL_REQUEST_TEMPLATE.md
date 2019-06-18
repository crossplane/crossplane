<!--
Thank you for helping to improve Crossplane!

We strongly recommend you look through our contributor guide at https://git.io/fj2m9
if this is your first time opening a Crossplane pull request. You can find us in
https://slack.crossplane.io/messages/dev if you need any help contributing.
-->

### Description of your changes
<!--
Briefly describe what this pull request does. Be sure to direct your reviewers'
attention to anything that needs special consideration.

We love pull requests that resolve an open Crossplane issue. If yours does, you
can uncomment the below line to indicate which issue your PR fixes, for example
"Fixes #500":

Fixes #
-->

### Checklist
<!--
Please run through the below readiness checklist. The first two items are
relevant to every Crossplane pull request.
-->
I have:
- [ ] Run `make reviewable` to ensure this PR is ready for review.
- [ ] Ensured this PR contains a neat, self documenting set of commits.
- [ ] Updated any relevant [documentation], [examples], or [release notes].
- [ ] Updated the RBAC permissions in [`clusterrole.yaml`] to include any new types.

[documentation]: https://github.com/crossplaneio/crossplane/tree/master/docs
[examples]: https://github.com/crossplaneio/crossplane/tree/master/cluster/examples
[release notes]: https://github.com/crossplaneio/crossplane/tree/master/PendingReleaseNotes.md
[`clusterrole.yaml`]: https://github.com/crossplaneio/crossplane/blob/master/cluster/charts/crossplane/templates/clusterrole.yaml