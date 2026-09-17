#!/usr/bin/env bash
# A live view of the composed resources, for projecting beside the terminal you
# type in. Shows creation order, readiness, and - during teardown - which
# resources are being deleted.
#
# Sorted by creation time, so new resources appear at the bottom as each wave
# is allowed to proceed.
set -euo pipefail

exec watch -n1 --no-title "
  echo 'XR:'
  kubectl -n default get xnetwork demo \
    -o custom-columns='NAME:.metadata.name,SYNCED:.status.conditions[?(@.type==\"Synced\")].status,READY:.status.conditions[?(@.type==\"Ready\")].status,MESSAGE:.status.conditions[?(@.type==\"Ready\")].message' \
    2>/dev/null || echo '  (no XR yet)'
  echo
  echo 'Composed resources:'
  kubectl -n default get nopresources.nop.crossplane.io \
    --sort-by=.metadata.creationTimestamp \
    -o custom-columns='RESOURCE:.metadata.annotations.crossplane\.io/composition-resource-name,READY:.status.conditions[?(@.type==\"Ready\")].status,DELETING:.metadata.deletionTimestamp' \
    2>/dev/null || echo '  (none yet)'
"
