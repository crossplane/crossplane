# Crossplane Governance

Crossplane follows two tier governance model. The higher level tier is made up of the
Crossplane Steering Committee that is responsible for the overall health of the project.
Maintainers and Organization Members make up the lower tier and they are the main
contributors to one or more repositories within the overall project.

The governance policies defined here apply to all repositories and work happening within
the following GitHub organizations:

* https://github.com/crossplane
* https://github.com/crossplane-contrib

## Steering Committee

The Crossplane Steering Committee oversees the overall health of the project. Its made
up of members that demonstrate a strong commitment to the project with views in the
interest of the broader Crossplane community. Their responsibilities include:

* Own the overall charter, vision, and direction of the Crossplane project
* Define and evolve project governance structures and policies, including
  project roles and how collaborators become maintainers.
* Promote broad adoption of the overall project, stewarding the growth and
  health of the entire community
* Provide guidance for the project maintainers
* Review and approve central/core architecture and design changes that have
  broad impact across multiple repositories
* Participate in the conflict resolution and voting process at the organization
  scope when necessary
* Approving new repositories, or archiving repositories in the Crossplane
  organizations
* Add and remove members in the Crossplane organizations
* Actively participate in the regularly scheduled steering committee meetings
* Regularly attend the recurring community meetings

### Composition

The Steering Committee has five (5) seats who will be selected through an election process
outlined below. Elected committee members will serve 2-year terms. The elections will be
staggered in order to preserve continuity - i.e. not all seats will be up for election at any
given year.

The following are some guidelines for eligibility to the Steering Committee:

* Steering committee members in many cases will already be a maintainer on one
    or more of the repositories within the Crossplane project and be
    contributing consistently.
* While an existing repository maintainer position is helpful, a proposed member
    should have also demonstrated broad architectural influence and
    contributions across diverse areas of the project.
* The proposed member should have consistently exhibited input for the good of
    the entire project beyond the organization they are affiliated with and
    their input must have been aligned with the general charter and strategy and
    always with the big picture in mind.
* There is **no** minimum time period for contributions to the project by the
  proposed member.

### Steering Committee

The members of the steering committee are listed in the table below (listed in
alphabetical order of first name). When terms of the committee members expire,
new members will be selected through the [election process](#election-process)
outlined below.

| &nbsp;                                                    | Member                                         | Organization | Email                       | Term Start | Term End   |
|-----------------------------------------------------------|------------------------------------------------|--------------|-----------------------------|------------|------------|
| <img width="30px" src="https://github.com/bassam.png">    | [Bassam Tabbara](https://github.com/bassam)    | Upbound      | bassam@upbound.io           | 2024-02-06 | 2026-02-06 |
| <img width="30px" src="https://github.com/bobh66.png">    | [Bob Haddleton](https://github.com/bobh66)     | Nokia        | bob.haddleton@nokia.com     | 2025-02-06 | 2027-02-06 |
| <img width="30px" src="https://github.com/lindblombr.png">| [Brian Lindblom](https://github.com/lindblombr)| Apple        | blindblom@apple.com         | 2025-02-06 | 2027-02-06 |
| <img width="30px" src="https://github.com/jbw976.png">    | [Jared Watts](https://github.com/jbw976)       | Upbound      | jared@upbound.io            | 2024-02-06 | 2026-02-06 |
| <img width="30px" src="https://github.com/negz.png">      | [Nic Cope](https://github.com/negz)            | Upbound      | negz@upbound.io             | 2024-02-06 | 2026-02-06 |

### Contact Info

The steering committee can be reached at the following locations:

* [`#steering-committee`](https://crossplane.slack.com/archives/C032WMA459S)
  channel on the [Crossplane Slack](https://slack.crossplane.io/) workspace
* [`steering@crossplane.io`](mailto:steering@crossplane.io) public email address

Members of the community as well as the broader ecosystem are welcome to contact
the steering committee for any issues or concerns they can assist with.

### Election Process

#### Eligibility for Voting

Voting for steering committee members is open to all current steering committee
members and [maintainers](#maintainers).

#### Nomination Criteria

* Each eligible voter can nominate up to 2 candidates
* Previous steering committee members are eligible to be nominated again
* An eligible voter can self-nominate
* Anyone can be nominated, they do **not** have to be an eligible voter
* The nominated candidate must accept the nomination in order to be included in
  the election
* Each nominated candidate must be endorsed by 2 eligible voters from 2
    different organizations (the candidate can self-endorse if they are eligible
    to vote)

#### Election

Elections will be held using time-limited
[Condorcet](https://en.wikipedia.org/wiki/Condorcet_method) ranking on
[CIVS](http://civs.cs.cornell.edu/) using the
[IRV](https://en.wikipedia.org/wiki/Instant-runoff_voting) method. The top vote
getters will be elected to the open seats.

#### Maximum Representation

When the initial steering committee terms expire, the maximum number of
steering committee members from any organization (or conglomerate, in the case of
companies owning each other), will be limited to 2 in order to encourage diversity.

If the results of an election result in greater than 2 members from a single
organization, the lowest vote getters from that organization will be removed and
replaced by the next highest vote getters until maximum representation on the
committee is restored.

If percentages shift because of job changes, acquisitions, or other events,
sufficient members of the committee must resign until the maximum representation
is restored. If it is impossible to find sufficient members to resign, the
entire company’s representation will be removed and new special elections held.
In the event of a question of company membership (for example evaluating
independence of corporate subsidiaries) a majority of all non-involved steering
committee members will decide.

#### Vacancies

In the event of a resignation or other loss of an elected steering committee
member, the candidate with the next most votes from the previous election will
be offered the seat. This process will continue until the seat is filled.

In case this fails to fill the seat, a special election for that position will
be held as soon as possible. Eligible voters from the most recent election will
vote in the special election (i.e., eligibility will not be redetermined at the
time of the special election). A committee member elected in a special election
will serve out the remainder of the term for the person they are replacing,
regardless of the length of that remainder.

## Repository Governance

The Crossplane project consists of multiple repositories that are published and
maintained as part of the [crossplane](https://github.com/crossplane) and
[crossplane-contrib](https://github.com/crossplane-contrib) organizations on
GitHub. Each repository will be subject to the same overall governance model,
but will be allowed to have different teams of people (“maintainers”) with
permissions and access to the repository.  This increases diversity of
maintainers in the Crossplane organization, and also increases the velocity
of code changes.

### Maintainers

Each repository in the Crossplane organizations are allowed their own unique set
of maintainers. Maintainers have the most experience with the given repo and are
expected to have the knowledge and insight to lead its growth and improvement.

In general, adding and removing maintainers for a given repo is the
responsibility of the existing maintainer team for that repo and therefore does
not require approval from the steering committee. However, in rare cases, the
steering committee can veto the addition of a new maintainer by following the
[conflict resolution process](#conflict-resolution-and-voting).

Responsibilities include:

* Strong commitment to the project
* Participate in design and technical discussions
* Participate in the conflict resolution and voting process at the repository
  scope when necessary
* Seek review and obtain approval from the steering committee when making a
    change to central architecture that will have broad impact across multiple
    repositories
* Contribute non-trivial pull requests
* Perform code reviews on other's pull requests
* Ensure that proposed changes to your repository adhere to the established
    standards, best practices, and guidelines, and that the overall quality and
    integrity of the code base is upheld.
* Add and remove maintainers to the repository as described below
* Approve and merge pull requests into the code base
* Regularly triage GitHub issues. The areas of specialization possibly listed in
    [OWNERS.md](OWNERS.md) can be used to help with routing an issue/question to
    the right person.
* Make sure that ongoing PRs are moving forward at the right pace or closing
  them
* Monitor Crossplane Slack (delayed response is perfectly acceptable),
    particularly for the area of your repository
* Regularly attend the recurring community meetings
* Periodically attend the recurring steering committee meetings to provide input
* In general continue to be willing to spend at least 25% of their time working
    on Crossplane (~1.25 business days per week)

The current list of maintainers for each repository is published and updated in
each repo’s [OWNERS.md](OWNERS.md) file.

### Becoming a Maintainer

To become a maintainer for a given repository, you need to demonstrate the
following:

* Consistently be seen as a leader in the Crossplane community by fulfilling the
    maintainer responsibilities listed above to some degree.
* Domain expertise in the area of focus for the repository
* Willingness to contribute and provide value to all areas of the repository’s
    code base, beyond simply the direct interests of your organization, and to
    consistently promote the overall charter and vision of the entire Crossplane
    organization.
* Be extremely proficient with the languages, tools, and frameworks used in the
  repository
* Consistently demonstrate:
  * Ability to write good solid code and tests
  * Ability to collaborate with the team
  * Understanding of how the team works (policies, processes for testing and
    code review, etc.)
  * Understanding of the project's code base and coding style

Beyond your contributions to the project, consider:

* If your organization already has a maintainer on a given repository, more
    maintainers from your org may not be needed. A valid reason, however, is
    "blast radius" to get more coverage on a large repository.
* Becoming a maintainer generally means that you are going to be spending
    substantial time (>25%) on Crossplane for the foreseeable future.

If you are meeting these requirements, express interest to the repository’s
existing maintainers directly.

* We may ask you to do some PRs from our backlog.
* As you gain experience with the code base and our standards, we will ask you
    to do code reviews for incoming PRs (i.e., all maintainers are expected to
    shoulder a proportional share of community reviews).
* After a period of approximately 2-3 months of working together and making sure
    we see eye to eye, the repository’s existing maintainers will confer and
    decide whether to grant maintainer status or not, as per the voting process
    described below. We make no guarantees on the length of time this will take,
    but 2-3 months is the approximate goal.
  * This time period is up to the discretion of the existing maintainer team and
        it is possible for new maintainers to be added in a shorter time period
        than this general guidance.

To formalize the addition of the new maintainer, the existing maintainer team
will then add them to the following locations:

* `OWNERS.md` file at the root of the repo
* The appropriate GitHub team that allows maintainer permissions to the repo,
  including merging pull requests into protected branches.
* CNCF maintainer lists
  * CNCF [project
    maintainers](https://github.com/cncf/foundation/blob/main/project-maintainers.csv)
    list
  * `cncf-crossplane-maintainers@lists.cncf.io` mailing list
  * See CNCF [new maintainer
    guidance](https://github.com/cncf/foundation/blob/main/.github/pull_request_template.md)
    for further details

#### Maintainers for New Repositories

The guidelines of collaborating for 2-3 months may not be feasible for when a
new repository is being created in the Crossplane organizations.  For new
repositories, the steering committee can choose to “bootstrap” the maintainer
team as they see fit.

### Removing a maintainer

If a maintainer is no longer interested or cannot perform the maintainer duties
listed above, they should volunteer to be moved to emeritus status. In extreme
cases this can also occur by a vote of the maintainers per the voting process
below.

The emeritus maintainer should be removed from all locations specified in the
[becoming a maintainer](#becoming-a-maintainer) section.

## Organization Members

Beyond the roles of the steering committee, and maintainers, outlined
above, contributors from the community can also be added to the Crossplane
organization as a “member”.  This affiliation only comes with the base
permissions (triage-only) for the organization, so the requirements are fairly
low.  Adding new members has the following benefits:

* Encourages a sense of belonging to the community and encourages collaboration
* Promotes visibility of the project by being displayed on each member’s profile
* Demonstrated tangible growth of the community and adoption of the project
* Allows issues to be assigned to the user

When adding a new member to the organization, they should meet some of the
following suggested requirements, which are open to the discretion of the
steering committee:

Community members who wish to become members of the Crossplane organization
should meet the following requirements, which are open to the discretion of the
steering committee:

* Have [enabled 2FA](https://github.com/settings/security) on their GitHub
  account.
* Have joined the [Crossplane slack channel](https://slack.crossplane.io/).
* Are actively contributing to the Crossplane project. Examples include:
  * opening issues
  * providing feedback on the project
  * engaging in discussions on issues, pull requests, Slack, etc.
  * attending community meetings
* Have reached out to two current Crossplane organization members who have
  agreed to sponsor their membership request.

Community members that want to join the organization should follow the [new
member
process](https://github.com/crossplane/org/blob/main/processes/new-member.md)
outlined in the `crossplane/org` repository. New members should be asked to set
the visibility of their Crossplane organization membership to public.

## Community Extension Projects

The [`crossplane-contrib`](https://github.com/crossplane-contrib) organization
serves as a vendor neutral home for community collaboration on crossplane
related projects and extensions. We commonly refer to the repositories in this
organization as **"community extension projects"** which includes providers,
functions and other crossplane extensions. As previously stated, this
organization and all of its repositories, in addition to the `crossplane`
organization, are under the governance and policies defined in this document.
They are effectively sub-projects of Crossplane and must follow all the same
rules and policies.

### Benefits of Becoming a Community Extension Project

It is entirely optional for a project in the greater Crossplane ecosystem to
formally join the `crossplane-contrib` organization as a community extension
project. The choice is up to the leadership team of each project. The benefits
of becoming a community extension project are listed below:

* To have a neutral home for collaboration with strong open source governance
  and licensing
* To attract more contributors and maintainers and increase the project’s long
  term sustainability
* To make the project eligible for reference in the Crossplane documentation,
  website, and publishing to the community `xpkg.crossplane.io` registry
* To contribute and participate more directly in the Crossplane community

### Policies for Community Extension Projects

All community extension projects are expected to adhere to the following set of
policies throughout their lifecycle:

* **CNCF Policies:** Each community extension project is part of the Crossplane
  CNCF project and must abide by all [CNCF
  policies](https://www.cncf.io/policies/).
* **Project Health**: Similarly, all extensions are expected to adhere to the
  [project health
  guidelines](https://contribute.cncf.io/maintainers/community/project-health/)
  published by the CNCF. As each extension will have varying needs, the health
  guidelines are not a completely prescriptive one size fits all approach, but
  all extensions must be operating with the spirit of these health guidelines in
  mind.
  * The steering committee is responsible for monitoring the state of these
    projects and taking action as defined in the [community extension project
    lifecycle](#community-extension-project-lifecycle) section below, including
    potential archival of the extension project.
  * Community members are welcome to bring any of their concerns or observations
    about a community extension project to the steering committee for review.
* **Registry:** Build artifacts, e.g. Crossplane packages (opinionated OCI
  images), must be published to the `crossplane-contrib` org on GitHub Container
  Registry (`ghcr.io`), which will serve as a neutral community registry.
  `ghcr.io` is preferred because it is integrated into GitHub, where all of the
  source code lives, and will reduce long term maintenance.
  * Optionally, the maintainer team can publish to additional registries of
    their choosing.
  * `xpkg.crossplane.io` will be published as a consistent entry point for pulls
    from the `ghcr.io/crossplane-contrib` organization. `xpkg.crossplane.io` is
    a Scarf gateway to help the project learn about adoption patterns through
    anonymized usage metrics, as
    [supported](https://contribute.cncf.io/resources/project-services/hosted-tools/)
    by the CNCF.
* **Maintainer list:** The current list of maintainers must be kept up to date
  in the following places:
  * `OWNERS.md` file at the root of the repo
  * CNCF [project
    maintainers](https://github.com/cncf/foundation/blob/main/project-maintainers.csv)
    list
  * `cncf-crossplane-maintainers@lists.cncf.io` mailing list
* **API group**: New extension projects starting out in `crossplane-contrib`
  should use  the  `*.crossplane.io`  API group for their API resources. If the
  project was started in a different organization and contributed to
  `crossplane-contrib`, it can maintain a different API group to avoid breaking
  all uses of the extension.

### Community Extension Project Lifecycle

To create a new community extension project, an issue must first be opened in
the [`crossplane/org`](https://github.com/crossplane/org) repository that
defines the following criteria:

* Name of new extension
* Purpose and intended scope of the extension
* Initial maintainer team (GitHub usernames)
* Description of the maintainer team's long term commitment to the extension

The steering committee will review the provided information and assess whether
the proposed extension is a good fit for the project. The steering committee may
provide feedback or ask for changes if they feel it is needed to improve the
chance of long term success of the extension. If approved, a new repository will
be created and the initial maintainers will be added.

Then, the day to day governance of the new extension repository follows the same
policies defined in this document for [repository
governance](#repository-governance) and [community extension project
policies](#policies-for-community-extension-projects). The maintainers operate in a
fairly autonomous fashion to maintain the repo and ensure its health and
success, and they can add/remove maintainers as defined.

#### Archival Policy

If a community extension project does not meet the policies defined in this
document, the steering committee should seek a resolution with the maintainer
team. Ideally, the maintainer team would be able to address the issues and bring
the extension back into good health and compliance. If the issues cannot be
resolved, the steering committee may decide to archive the repository. When a
repository is archived, this status change should be made very clear to the
community to reduce any potential confusion. Any references to the project from
the Crossplane documentation and/or website will be removed at this time.

### Public References to Community Extension Projects

Crossplane's public facing content, such as the [crossplane.io
website](https://www.crossplane.io/) and Crossplane
[documentation](https://docs.crossplane.io/), should only reference or promote
community extension projects that are in good health and compliance with all
[policies](#policies-for-community-extension-projects). For example, the docs
site should not include guides for extensions that are private, owned by
vendors, or archived. All public references by the project must be only for
healthy and compliant community extension projects.

## Updating the Governance

This governance will likely be a living document and its policies will therefore
need to be updated over time as the community grows.  The steering committee has
full ownership of this governance and only the committee may make updates to it.
Changes can be made at any time, but a **super majority** (at least **2/3** of
votes) is required to approve any updates.

## Conflict resolution and voting

In general, it is preferred that technical issues and maintainer membership are
amicably worked out between the persons involved. If a dispute cannot be decided
independently, the leadership at the appropriate scope can be called in to
decide an issue. If that group cannot decide an issue themselves, the issue will
be resolved by voting.

### Issue Voting Scopes

Issues can be resolved or voted on at different scopes:

* **Repository**: When an issue or conflict only affects a single repository,
    then the **maintainer team** for that repository should resolve or vote on
    the issue.  This includes technical decisions as well as maintainer team
    membership.
* **Organization**: If an issue or conflict affects multiple repositories or the
    Crossplane organizations and community at large, the **steering committee**
    should resolve or vote on the issue.

### Issue Voting Process

The issue voting process is usually a simple majority in which each entity
within the voting scope gets a single vote. The following decisions require a
**super majority** (at least **2/3** of votes), all other decisions and changes
require only a simple majority:

* Updates to governance by the steering committee
* Additions and removals of maintainers by the repository’s current maintainer
  team
* Vetoes of maintainer additions by the steering committee

For organization scope voting, repository maintainers do not have a vote in this
process, although steering committee members should consider their input.

For formal votes, a specific statement of what is being voted on should be added
to the relevant GitHub issue or PR. Voting entities should indicate their yes/no
vote on that issue or PR.

After a suitable period of time (goal is by 5 business days), the votes will be
tallied and the outcome noted. If any voting entities are unreachable during the
voting period, postponing the completion of the voting process should be
considered.
