# TODO(negz): This is dumb. Don't build crun every time we build Crossplane.
FROM nixos/nix:2.12.0 AS crun-build

ARG CRUN_VERSION=1.7.2

RUN git clone --branch ${CRUN_VERSION} --recursive https://github.com/containers/crun
WORKDIR crun
RUN nix --extra-experimental-features nix-command build -f nix --arg enableSystemd false

FROM gcr.io/distroless/static@sha256:d2b0ec3141031720cf5eedef3493b8e129bc91935a43b50562fbe5429878d96b

ARG TARGETOS
ARG TARGETARCH

COPY --from=crun-build /crun/result/bin/crun /usr/local/bin/
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
