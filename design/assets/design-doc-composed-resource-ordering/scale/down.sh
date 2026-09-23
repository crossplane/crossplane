#!/usr/bin/env bash
# Delete the scale testing cluster.
set -euo pipefail
cluster="${CLUSTER:-xp-scale}"
if command -v kind >/dev/null; then
	kind delete cluster --name "${cluster}"
else
	nix run nixpkgs#kind -- delete cluster --name "${cluster}"
fi
