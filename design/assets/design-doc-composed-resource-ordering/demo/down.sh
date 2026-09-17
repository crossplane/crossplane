#!/usr/bin/env bash
# Delete the demo cluster.
set -euo pipefail
kind delete cluster --name "${CLUSTER:-xp-ordering-demo}"
