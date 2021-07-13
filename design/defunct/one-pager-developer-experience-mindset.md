# Developer experience mindset and scope

* Owner: Daniel Suskin (@suskin)
* Reviewers: Crossplane Maintainers
* Status: Defunct

How do we think about developer experience? What does "developer
experience" mean?


## Abstract
The goal of having a document about developer experience mindset is to
explain and document our thinking around developer experiences. This
allows us to more easily keep the mindset in mind, teach new
contributors what our mindset is, and share our mindset with people who
may be interested in it.

The short version of the mindset for developer experiences is that for a
given tool:
* The tool should be fun to use.
* People should want to use the tool.
* People should feel supported when using the tool.

The short version of the scope of what "developer experience" means in
the context of Crossplane is:
* Making the lives of application authors and application operators
  easier.
* Helping to foster effective relationships between different types of
  users of a tool. For example, in our case, a cloud admin and an
  application operator.


## Mindset
Developer experience encompasses a certain mindset which is different
from some other types of software development, so it's worth explaining
what the mindset is.

Developer experience work is very user-oriented, and is well-served by
user-oriented thinking. Here are some fundamental principles for a
project in the developer experience space.

### It should be fun to use
Developing software should be enjoyable. Any time there's a pain point
when developing (in this case against Kubernetes), that's an opportunity
to improve the developer experience to make things more enjoyable.

A fun experience implies a certain level of quality: things should Just
Work out of the box. There shouldn't be a ton of bugs, and things should
be stable most of the time. The size of the improvement a tool provides
should be large, either in convenience or functionality. Common tasks
should be easy to do, and advanced tasks should be possible.
Functionality should be discoverable.

### People should want to use it
Wanting to use a tool gets back to quality and fun, so see "It should be
fun to use" as a starting point. But it also informs how we interact
with users: how do we know if people want to use it unless we ask? We
should be constantly striving for feedback, and always trying to improve
on what users don't like. If users are only using a tool because it's
the only option available, that isn't good enough.

### People should feel supported when using it
People should have a good feeling about whether they feel supported in
their use of a project. On some level, this means the tool should be
kept up-to-date, and that bugs should be fixed in a timely manner. On
another level, this means that people should feel good about interacting
with the community: they should feel respected when they reach out; they
should feel listened to when they raise concerns; and they should feel
taken care of when they have problems. Users with knowledge gaps should
be nurtured.


## Scope
The term "developer experience" can have multiple interpretations. Who
is the developer? What is an experience? We have certain definitions in
mind.

There are multiple different "users" in the space that we're in, which
we have been calling personas:
* The cloud admin, who is running a Kubernetes cluster.
* The application author, who is writing applications which can be run
  on a Kubernetes cluster. These applications may or may not consume
  other applications.
* The application operator, who is running applications running on a
  Kubernetes cluster.
* The security person, who is ensuring that security policies exist and
  are enforced.
* The consumer of a running application. This is the "end user" of an
  application running on a Kubernetes cluster.

In this context, developer experience refers to making the lives of the
application author and application operator easier.

It also applies to the relationship between different personas which are
using the tool. For example, ideally, a tool would be able to help make
the relationship between a cloud admin and an application operator very
smooth.
