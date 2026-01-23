#!/usr/bin/env bash
# nix.sh - Run Nix commands via Docker without installing Nix locally.
#
# Usage: ./nix.sh <command>
#
# Run './nix.sh flake show' for available apps and packages, or see flake.nix.
# Examples: ./nix.sh run .#test, ./nix.sh build, ./nix.sh develop
#
# The first run downloads dependencies into /nix/store (cached in a Docker
# volume). Subsequent runs reuse the cache. To reset: docker volume rm crossplane-nix

set -e

# When NIX_SH_CONTAINER is set, we're running inside the Docker container.
# This script re-executes itself inside the container to avoid sh -c quoting.

if [ "${NIX_SH_CONTAINER:-}" = "1" ]; then
  # Install tools this entrypoint script needs. It needs Docker to setup Docker
  # in Docker for E2E tests, and rsync to copy build the build result (cp
  # doesn't work well on MacOS volumes). Installed packages persist across runs
  # thanks to the crossplane-nix volume.
  nix-env -iA nixpkgs.docker nixpkgs.rsync

  # Start the Docker daemon, storing its data in the crossplane-nix volume for
  # persistence across container runs. This gives us a consistent Docker
  # environment with cached images (e.g., kind node images).
  dockerd --data-root=/nix/docker >/tmp/dockerd.log 2>&1 &

  attempts=0
  until docker info >/dev/null 2>&1; do
    sleep 1
    attempts=$((attempts + 1))
    if [ ${attempts} -gt 30 ]; then
      echo "Docker failed to start. Logs:"
      cat /tmp/dockerd.log
      exit 1
    fi
  done

  # The container runs as root, but the bind-mounted /crossplane is owned by
  # the host user. Git refuses to operate in directories owned by other users.
  git config --global --add safe.directory /crossplane

  # Record the current time. After nix runs, we'll find files newer than this
  # marker and chown them to the host user.
  marker=$(mktemp)

  # If result (i.e. the build output) is a directory, remove it so nix build can
  # create its symlink. We only remove directories, not symlinks (which might be
  # from a host Nix install).
  if [ -d result ] && [ ! -L result ]; then
    rm -rf result
  fi

  nix "${@}"

  # Nix build makes result/ a symlink to a directory in the Nix store. That
  # directory only exists inside the container, but it creates the symlink in
  # /crossplane, which is shared with the host. We use this rsync trick to make
  # result/ a directory of regular files.
  if [ -L result ] && readlink result | grep -q '^/nix/store/' && [ -e result ]; then
    rsync -rL --chmod=u+w result/ result.tmp
    rm result
    mv result.tmp result
  fi

  # Fix ownership of any files nix created or modified. The container runs as
  # root, so without this, generated files would be root-owned on the host.
  # Using -newer is surgical - we only chown files touched during this run.
  find /crossplane -newer "${marker}" -exec chown "${HOST_UID}:${HOST_GID}" {} + 2>/dev/null || true
  rm -f "${marker}"

  exit 0
fi

# When running on the host, launch a Docker container and re-execute this
# script inside it.

# Nix configuration, equivalent to /etc/nix/nix.conf.
NIX_CONFIG="
# Flakes are Nix's modern project format - a flake.nix file plus a flake.lock
# that pins all dependencies. This is still marked 'experimental' but is stable
# and widely used.
experimental-features = nix-command flakes

# Build multiple derivations in parallel. A derivation is Nix's build unit,
# like a Makefile target. 'auto' uses one job per CPU core.
max-jobs = auto

# Cachix is a binary cache service. Our GitHub Actions CI pushes there, so if CI
# has recently built the commit you're on Nix will download stuff instead of
# rebuilding it locally.
extra-substituters = https://crossplane.cachix.org
extra-trusted-public-keys = crossplane.cachix.org-1:NJluVUN9TX0rY/zAxHYaT19Y5ik4ELH4uFuxje+62d4=
"

# Only allocate a TTY if stdout is a terminal. TTY mode corrupts binary output
# (e.g., when piping stream-image to docker load). The -i flag keeps stdin open
# for interactive commands like 'nix develop'.
INTERACTIVE_FLAGS=""
if [ -t 1 ]; then
  INTERACTIVE_FLAGS="-it"
fi

# Run with --privileged for Docker-in-Docker (required for kind clusters).
#
# We use true Docker-in-Docker (a Docker daemon inside the container) rather
# than Docker-outside-of-Docker (mounting /var/run/docker.sock). DinD avoids
# path translation issues: when kind tells Docker to mount a path, that path
# must exist where Docker runs. With DooD, Docker runs on the host but the
# paths only exist in the container.
docker run --rm --privileged ${INTERACTIVE_FLAGS} \
  -v "$(pwd):/crossplane" \
  -v "crossplane-nix:/nix" \
  -w /crossplane \
  -e "NIX_SH_CONTAINER=1" \
  -e "NIX_CONFIG=${NIX_CONFIG}" \
  -e "GOMODCACHE=/nix/go-mod-cache" \
  -e "GOCACHE=/nix/go-build-cache" \
  -e "HOST_UID=$(id -u)" \
  -e "HOST_GID=$(id -g)" \
  nixos/nix \
  /crossplane/nix.sh "${@}"
