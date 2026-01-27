#!/bin/bash

set -e

# Install Earthly (for release branches)
echo "Installing Earthly..."
curl -fsSLo /usr/local/bin/earthly https://github.com/earthly/earthly/releases/latest/download/earthly-linux-amd64
chmod +x /usr/local/bin/earthly
/usr/local/bin/earthly bootstrap

# Install Nix (for main branch)
echo "Installing Nix..."
apt-get update && apt-get install -y nix-bin

# Configure Nix via environment variable
export NIX_CONFIG="
experimental-features = nix-command flakes
max-jobs = auto
extra-substituters = https://crossplane.cachix.org
extra-trusted-public-keys = crossplane.cachix.org-1:NJluVUN9TX0rY/zAxHYaT19Y5ik4ELH4uFuxje+62d4=
"

echo "Nix $(nix --version) installed successfully"

renovate
