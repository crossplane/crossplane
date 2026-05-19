#!/bin/bash

set -e

# Install Nix.
echo "Installing Nix..."
apt-get update && apt-get install -y nix-bin

# Configure Nix
mkdir -p /etc/nix
cat >/etc/nix/nix.conf <<'EOF'
# Enable flakes and the nix command (e.g. nix run, nix build).
experimental-features = nix-command flakes

# Run builds as the calling user, not dedicated nixbld users. This avoids
# needing to create the nixbld group and users in this ephemeral container.
build-users-group =

# Build derivations in parallel, one per CPU core.
max-jobs = auto

# Use the Crossplane Cachix cache to download pre-built binaries from CI.
extra-substituters = https://crossplane.cachix.org
extra-trusted-public-keys = crossplane.cachix.org-1:NJluVUN9TX0rY/zAxHYaT19Y5ik4ELH4uFuxje+62d4=
EOF

echo "Nix $(nix --version) installed successfully"

# Install Earthly (for release branches) from the repository flake on main,
# pinned by main's flake.lock. The flake is referenced by URL because this
# entrypoint runs in the Renovate container before the target repo is checked
# out, so the working directory does not yet contain a flake.nix.
echo "Installing Earthly..."
nix profile install github:crossplane/crossplane#earthly
earthly bootstrap

renovate
