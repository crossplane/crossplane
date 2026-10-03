# Crossplane AI Contribution Policy

This policy defines how the Crossplane team expects contributors to engage with the project while
using AI tools like agents and LLMs.

This policy applies to all repositories in the `crossplane` and `crossplane-contrib` GitHub
organizations, just like the project [governance](GOVERNANCE.md).

## AI usage is assumed

The Crossplane team makes use of AI coding tools every day and we expect that almost all
contributors will be doing the same. Therefore, we do not require any disclosure for the usage of AI
in any contributions.

We discourage including attribution for your agent in your commits, e.g., a `Co-authored-by`
trailer, as it pollutes contributor statistics.

It is more important that we have high quality contributions and engagement with contributors, so
this policy focuses on achieving that goal.

## Own and understand your contribution

Contributors are responsible for the entirety of their submission, no matter what tooling they used
to produce it.

- **Understand every line you are changing:** If you cannot explain the purpose and impact of every
  change without the assistance of an agent, it's not ready for submission.
- **Think and verify critically:** Review and test your entire change deeply, both when opening a
  new PR and updating an existing one in response to feedback. The maintainer team should not be the
  first human to start critiquing or verifying your changes.
- **Use the templates:** PRs and issues have templates, so use them and provide the information they
  are requesting. An incoming PR that doesn't even contain the template's checklist is a clear sign
  your agent opened the PR for you and you didn't take much care during the PR creation process.
- **Don't blame your agent:** "My agent did/wrote that" is not a sufficient answer to review
  comments, it's critical to understand your changes more deeply than that.

AI tools are very capable of producing plausible looking changes for problems they don't deeply
understand. It's your job as a contributor to own, understand, and verify your changes to a high
degree of confidence.

## Talk to us yourself

### Express your reasoning naturally

We want to engage with human contributors, we do not want to feel like we are just talking to your
agent. We have our own agents that we can talk to and we can do that much faster than going through
you. We'd much rather connect directly and intellectually with the human behind the submission.

So please share your own reasoning behind your changes, e.g., why you framed the problem this way,
what you tried, what you are unsure about, or details about your use case that necessitates this
change.

### Use the right level of detail

Your writing should not make it **harder** to understand you. AI tools love to generate enormous
walls of text that prematurely go into an exorbitant level of detail, which is challenging for the
maintainer team to read and fully understand. These details often obscure the core problem and make
it more challenging to align on the right path forward together.

Code comments should help a future reader understand the reasoning behind implementation choices
that may be subtle or difficult to see just from reading the code itself. Agents tend to write for
the reviewer instead, narrating the alternatives they rejected or explaining why their change is
correct. That context may be helpful while we review but becomes clutter once the change is merged,
so it belongs in your pull request description instead of the codebase.

### Write naturally

We highly encourage you to write your own issues, pull request descriptions, and comment replies.
It's fine to use tools to help you get the words right, especially if English is not your first
language, but the thinking has to be yours, and it should read like it. Blindly pasting clearly
generated text while engaging with a maintainer is not the type of interaction we're looking for.

### Review with your own judgment

Reviewing pull requests is one of the best ways to learn this codebase, and it's a path toward
becoming a maintainer, so we appreciate more people doing it. But that depends on you being the one
doing the reviewing.

Don't point an agent at someone else's pull request and just post what it produces. We already have
automated AI reviewing tools set up and don't need your agents fabricating additional engagement.

Using AI to help you understand a change you're reviewing is fine, but please make sure the review
you post is your own.

## Maintainer time is limited

Time and energy from the human maintainers on this project are a limited resource. AI has made
writing code and opening a pull request a very cheap operation, which can easily overwhelm the team
responsible for reviewing and supporting those changes.

- **Bias towards discussion first:** For anything beyond a clearly scoped fix, we encourage you to
  open an issue and drive alignment on the direction before implementing the required changes. This
  is not a hard requirement, but it is an effective way to avoid spending cycles on a change that we
  won't accept.
- **Keep changes small and focused:** This reduces the effort of reviewing the change and ensuring
  its quality and makes it more likely to be accepted.
- **Avoid flooding us with changes:** If you already have pull requests open that have not been
  merged, avoid opening more pull requests that continue to grow the review backlog. This is most
  important when you are a new contributor and we haven't yet built a trusting relationship.

## Enforcement

The maintainer team may close pull requests and issues that do not meet the spirit of the guidelines
in this policy. We may even do so without first writing a detailed explanation or providing a
technical critique. We will most likely not debate whether a contribution violated this policy.

Contributors who continue to violate this policy will be blocked from further engagement in the
project. Closing submissions is not the only tool we have, and we will use the others when we need
to.

This policy is not aimed at contributors who want to learn the project and authentically engage with
the team. That's exactly the experience we are seeking to have. If you are new and trying to
understand something, then we are happy to help and collaborate in meaningful ways with other
humans.

## Legal

Crossplane is a CNCF project, which is part of the Linux Foundation. Therefore, the LF's [Generative
AI policy](https://www.linuxfoundation.org/legal/generative-ai) applies to every contribution and
must be adhered to.