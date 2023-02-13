# This is debian:bookworm-slim (i.e. Debian 12, testing), which has crun v1.5.
FROM debian:bookworm-slim@sha256:72cc75fa1097aa604b310e70fee7e19afa24d8b64057cc6a717066207af29ee3

ARG TARGETOS
ARG TARGETARCH

# TODO(negz): Find a better way to get an OCI runtime? Ideally we'd grab a
# static build of crun (or runc) that we could drop into a distroless image. We
# slightly prefer crun for its nascent WASM and KVM capabilities, but they only
# offer static builds for amd64 and arm64 and building our own takes a long
# time.
RUN apt-get update && apt-get install -y ca-certificates crun && rm -rf /var/lib/apt/lists/*

ADD bin/${TARGETOS}\_${TARGETARCH}/xfn /usr/local/bin/

# We run xfn as root in order to grant it CAP_SETUID and CAP_SETGID, which are
# required in order to create a user namespace with more than one available UID
# and GID. xfn invokes all of the logic that actually fetches, caches, and runs
# a container as an unprivileged user (relative to the root/initial user
# namespace - the user is privileged inside the user namespace xfn creates).
# 
# It's possible to run xfn without any root privileges at all - uncomment the
# following line to do so. Note that in this mode xfn will only be able to
# create containers with a single UID and GID (0), so Containerized Functions
# that don't run as root may not work.
# USER 65532

ENTRYPOINT ["xfn"]
