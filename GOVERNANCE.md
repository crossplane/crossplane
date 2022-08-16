# Crossplane Governance

This document defines governance policies for the Crossplane project, which is
developed across two GitHub organizations:

* https://github.com/crossplane
* https://github.com/crossplane-contrib

Both GitHub organizations fall under Crossplane's governance policies, but only
the crossplane organization is constrained by its [charter](CHARTER.md). The
crossplane-contrib organization exists to provide a neutral home for Crossplane
extensions. Such extensions fall outside of the project's charter and are not
required to fall under the project's governance, but may do so by choosing to be
hosted in the crossplane-contrib organization.

## Steering Committee

Steering committee members demonstrate a strong commitment to the project with
views in the interest of the broader Crossplane community. They are the stewards
of the entire Crossplane organization and are expected to dedicate thoughtful
and serious effort towards the goal of general success in the ecosystem.

Responsibilities include:

* Own the overall charter, vision, and direction of the Crossplane project
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

The current list of steering committee members is published and updated in
[OWNERS.md](OWNERS.md). All steering committee members will be made owners of
the Crossplane organizations in GitHub.

### Bootstrapping the steering committee

The membership of the steering committee will go through an initial
bootstrapping process to help transition the stewardship and leadership of the
community over a clearly defined time period and process while ensuring that the
project remains true to its initial charter.  It is essential that the existing
vision and strategy continues to guide the project while it continues to grow.
Therefore, transition in stewardship should not happen abruptly and the founding
leadership should not be diluted too quickly while the project is still being
bootstrapped and transitioned.

Therefore, the membership will be bootstrapped over a **2 year period**, from
the date of acceptance of this governance, as follows:

* Membership will be **limited to 5** total seats
* **3 seats** are granted to the senior maintainers from the existing Crossplane
    governance: Bassam Tabbara, Jared Watts, and Nic Cope
  * **2 year** term
* **2 seats** to be filled by members of the community selected by the above
  committee members
  * **1 year** term
* When the term for a seat expires, the election process will be used to fill
    the seat with a replacement member with a **2 year** term

**After** the bootstrapping period of **2 years** has ended, the following
membership rules will become enacted:

* All terms are valid for **2 years**
* A single organization may have no more than **2 seats** on the committee

#### Updates During the Bootstrapping Period

We recognize that the bootstrapping period as defined above may not efficiently
serve the needs of the community as it experiences more growth and contributors,
especially if that growth is rapid. It is very difficult to forecast how the
community will grow, but we do believe that the constraints of the given
bootstrapping period are a sufficient starting point.

We earnestly intend for the bootstrapping period to appropriately serve the
needs of the community. If the community expands in a manner that outgrows the
limitations of the steering committee, then we fully intend to make necessary
changes, such as adding new seats to the committee in an out-of-band process
from the election schedule described in this governance.

The steering committee will have final say on when and how the committee is
grown, but we intend for it to be allowed to grow **if and when** the needs of
the community require it. Any changes to the bootstrapping period will require a
**super majority** (at least **2/3** of votes) as described in the [updating the
governance](#updating-the-governance) section.

### Membership Qualifications

This section outlines the desired qualifications to become a member of the
steering committee. Since members will be selected through the election process
after the initial bootstrapping, these qualifications are meant to serve more as
guidance and education than as rigid rules.

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

### Election Process

#### Eligibility for Voting

Voting for steering committee members is open to all current steering committee
members and repository maintainers.

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

As previously stated, when the bootstrapping period ends, the maximum number of
steering committee members from any organization will be limited to 2 in order
to encourage diversity.

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
GitGub. Each repository will be subject to the same overall governance model,
but will be allowed to have different teams of people (“maintainers”) with
permissions and access to the repository.  This increases diversity of
maintainership in the Crossplane organization, and also increases the velocity
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
existing maintainers directly. Becoming a [reviewer](#reviewers) is a great way
to work up to becoming a maintainer.

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

The existing maintainer team will then add the new maintainer to the repo’s
[OWNERS.md](OWNERS.md) file, as well as the appropriate GitHub team that allows
maintainer permissions to the repo, including merging pull requests into
protected branches.

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

## Reviewers

Each repository in the Crossplane organizations may also have their own unique
set of reviewers. Reviewers help maintainers review new contributions. They're
typically newer to the project and interested in working toward becoming a
maintainer. Reviewers may approve but not merge PRs - all PRs must be approved
by a maintainer.

In general, adding and removing reviewers for a given repo is the
responsibility of the existing maintainer team for that repo and therefore does
not require approval from the steering committee. However, in rare cases, the
steering committee can veto the addition of a new reviewer by following the
[conflict resolution process](#conflict-resolution-and-voting).

Responsibilities include:

* Strong commitment to the project.
* Participate in design and technical discussions.
* Perform 2-3 code reviews per week on other's pull requests.
* Ensure that proposed changes to your repository adhere to the established
  standards, best practices, and guidelines, and that the overall quality and
  integrity of the code base is upheld.
* Ensure maintainers have the context they need to merge pull requests.
* Make sure that ongoing PRs are moving forward at the right pace or closing
  them.
* Monitor Crossplane Slack (delayed response is perfectly acceptable),
  particularly for the area of your repository
* Regularly attend the recurring community meetings.

The current list of reviewers for each repository (if any) is published and
updated in each repo’s [OWNERS.md](OWNERS.md) file.

### Becoming a Reviewer

To become a maintainer for a given repository, you need to demonstrate the
following:

* Be familiar with the languages, tools, and frameworks used in the repository.
* Be deeply familiar and aligned with the project's
  [CONTRIBUTING.md](CONTRIBUTING.md) guide.
* Consistently demonstrate:
  * Ability to collaborate with the team
  * Understanding of how the team works (policies, processes for testing and
    code review, etc.)
  * Understanding of the project's code base and coding style

If you are meeting these requirements, express interest to the repository’s
maintainers directly. The maintainer team will then add the new reviewer to the
repo’s [OWNERS.md](OWNERS.md) file, as well as the appropriate GitHub team that
allows write permissions to the repo, including approving pull requests.

#### Reviewers for New Repositories

For new repositories, the maintainers can choose to “bootstrap” the reviewer
team as they see fit.

### Removing a Reviewer

If a reviewer is no longer interested or cannot perform the reviewer duties
listed above, they should volunteer to be moved to emeritus status. In extreme
cases this can also occur by a vote of the maintainers per
[the voting process](#conflict-resolution-and-voting).

## Organization Members

Beyond the roles of the steering committee, maintainers, and reviewers outlined
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
