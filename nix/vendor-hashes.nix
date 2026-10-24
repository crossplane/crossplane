# Go vendor hashes for buildGoModule, one per Go module.
#
# This file is the buildGoModule equivalent of the old gomod2nix.toml files: it
# pins the hash of each module's vendored dependencies so builds stay
# reproducible inside the Nix sandbox.
#
# Regenerate after changing Go dependencies with:
#
#   nix run .#tidy
#
# (tidy runs `go mod tidy` and then rewrites the hashes below. Don't edit them
# by hand.)
{
  # Root module: github.com/crossplane/crossplane/v2
  root = "sha256-pBwWTev+uB/K9TV9fReJLzzH2P3JDViv6jGoSAeR4HI=";
}
