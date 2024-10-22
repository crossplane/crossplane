# Security Policy

This policy is adapted from the policies of the following CNCF projects:

* [Rook](https://github.com/rook/rook)
* [Containerd](https://github.com/containerd/project)

## Audits

The following security related audits have been performed in the Crossplane
project and are available for download from the [security folder](./security)
and from the direct links below:

* A security audit was completed in July 2023 by [Ada
  Logics](https://adalogics.com/). The full report is available
  [here](./security/ADA-security-audit-23.pdf).
* A fuzzing security audit was completed in March 2023 by [Ada
  Logics](https://adalogics.com/). The full report is available
  [here](./security/ADA-fuzzing-audit-22.pdf).

## Reporting a Vulnerability

To report a vulnerability, either:

1. Report it on Github directly you can follow the procedure described
   [here](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability)
   and:

    - Navigate to the [security tab](https://github.com/crossplane/crossplane/security) on the repository
    - Click on 'Advisories'
    - Click on 'Report a vulnerability'
    - Detail the issue, see below for some expamples of info that might be
      useful including.

2. Send an email to `crossplane-security@lists.cncf.io` detailing the issue,
   see below for some examples of info that might be useful including.

The reporter(s) can typically expect a response within 24 hours acknowledging
the issue was received. If a response is not received within 24 hours, please
reach out to any
[maintainer](https://github.com/crossplane/crossplane/blob/main/OWNERS.md#maintainers)
directly to confirm receipt of the issue.

### Report Content

Make sure to include all the details that might help maintainers better
understand and prioritize it, for example here is a list of details that might be
worth adding:

- Versions of Crossplane used and more broadly of any other software involved,
  e.g. Kubernetes, providers, ...
- Detailed list of steps to reproduce the vulnerability.
- Consequences of the vulnerability.
- Severity you feel should be attributed to the vulnerabilities.
- Screenshots, logs or Kubernetes Events

Feel free to extend the list above with everything else you think would be
useful.

## Review Process

Once a maintainer has confirmed the relevance of the report, a draft security
advisory will be created on Github. The draft advisory will be used to discuss
the issue with maintainers, the reporter(s), and Crossplane's security advisors.
If the reporter(s) wishes to participate in this discussion, then provide
reporter Github username(s) to be invited to the discussion. If the reporter(s)
does not wish to participate directly in the discussion, then the reporter(s)
can request to be updated regularly via email.

If the vulnerability is accepted, a timeline for developing a patch, public
disclosure, and patch release will be determined. If there is an embargo period
on public disclosure before the patch release, an announcement will be sent to
the security announce mailing list (`crossplane-security-announce@lists.cncf.io`)
announcing the scope of the vulnerability, the date of availability of the
patch release, and the date of public disclosure. The reporter(s) are expected
to participate in the discussion of the timeline and abide by agreed upon dates
for public disclosure.

## Public Disclosure Process

Vulnerabilities once fixed, will be disclosed via email to
`crossplane-security-announce@lists.cncf.io`, shared publicly as a Github [security
advisory](https://docs.github.com/en/code-security/security-advisories/repository-security-advisories/about-repository-security-advisories)
and mentioned in the fixed versions' release notes.

## Supported Versions

See [Crossplane's documentation](https://docs.crossplane.io/latest/learn/release-cycle/)
for information on supported versions of crossplane. Any supported
release branch may receive security updates. For any security issues discovered
on older versions, non-core packages, or dependencies, please inform maintainers
using the same security mailing list as for reporting vulnerabilities.

## Joining the security announce mailing list

The security announcement mailing list
`crossplane-security-announce@lists.cncf.io`, will be used to announce
vulnerabilities, often ahead of when a fix is made available, to a restricted
set of Crossplane adopters and vendors.

Any organization or individual who directly uses crossplane and non-core
packages in production or in a security critical application is eligible to
join the security announce mailing list. Indirect users who use crossplane
through a vendor are not expected to join, but should request their vendor
join. To join the mailing list, the individual or organization must be
sponsored by either a crossplane maintainer or security advisor as well as have
a record of properly handling non-public security information. If a sponsor
cannot be found, sponsorship may be requested at
`crossplane-security-announce@lists.cncf.io`. Sponsorship should not be
requested via public channels since membership of the security announce list is
not public.
