#!/usr/bin/env bash
# Install metrics-server on the scale cluster, so a run can report what
# Crossplane and the provider actually used rather than inferring it from
# whether raising a limit changed the timing.
#
# --kubelet-insecure-tls is required on kind: the kubelet's serving
# certificate isn't signed by the cluster CA, so metrics-server refuses to
# scrape it otherwise, and every `kubectl top` returns nothing.
#
# Pinned rather than latest: a scale measurement that can't be repeated later
# isn't worth much.
set -euo pipefail

version="${METRICS_SERVER_VERSION:-v0.7.2}"

echo "==> installing metrics-server ${version}"
kubectl apply -f "https://github.com/kubernetes-sigs/metrics-server/releases/download/${version}/components.yaml"

kubectl -n kube-system patch deploy metrics-server --type=json -p '[
 {"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}
]'

kubectl -n kube-system rollout status deploy/metrics-server --timeout=3m

echo "==> waiting for the first scrape"
for _ in $(seq 1 30); do
	if kubectl top pods -n crossplane-system >/dev/null 2>&1; then
		kubectl top pods -n crossplane-system
		exit 0
	fi
	sleep 5
done

echo "!! metrics-server is up but kubectl top still returns nothing" >&2
exit 1
