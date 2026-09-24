#!/usr/bin/env bash
# Stand up a kind cluster running Crossplane from this branch, for scale and
# timing runs against real composed resources.
#
# This is the demo's setup, with the image built through Nix rather than a
# Docker build: the machine that runs scale tests has Nix and need not have a
# Go toolchain.
#
# The realtime compositions circuit breaker is taken out of the way by
# default. It rate-limits watch-driven reconciles per XR, and ordering
# converges over many reconciles, so a graph of ten resources can trip it -
# after which each wave waits for a half-open probe and the measurement
# describes the breaker rather than the ordering. Raising the burst far above
# any run's event count keeps it closed; it changes nothing about how ordering
# works. Separate findings on the breaker itself are in
# notes-circuit-breaker-scale-findings.md.
#
# Usage:
#   ./up.sh
#   CROSSPLANE_ARGS="--circuit-breaker-burst=100" ./up.sh   # study the breaker
#   CROSSPLANE_ARGS=" " ./up.sh                             # stock defaults
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "${here}/../../../.." && pwd)"
cluster="${CLUSTER:-xp-scale}"
extra_args="${CROSSPLANE_ARGS:---circuit-breaker-burst=100000}"

command -v nix >/dev/null || {
	echo "nix is not on PATH. On the test VM: export PATH=/nix/var/nix/profiles/default/bin:\$PATH" >&2
	exit 1
}

# kind and helm normally reach a Crossplane checkout through the flake's apps,
# so a machine that has Nix need not have either installed. Fall back to
# running them from nixpkgs rather than asking for a manual install.
if command -v kind >/dev/null; then
	KIND=(kind)
else
	KIND=(nix run nixpkgs#kind --)
fi

if command -v helm >/dev/null; then
	HELM=(helm)
else
	HELM=(nix run nixpkgs#kubernetes-helm --)
fi

echo "==> creating kind cluster ${cluster}"
"${KIND[@]}" delete cluster --name "${cluster}" >/dev/null 2>&1 || true
"${KIND[@]}" create cluster --name "${cluster}"

echo "==> installing a released Crossplane, for its CRDs, RBAC and TLS secrets"
kubectl create namespace crossplane-system
"${HELM[@]}" install crossplane "${repo}/cluster/charts/crossplane" \
	-n crossplane-system \
	--set image.repository=crossplane/crossplane --set image.tag=v2.4.0 \
	--wait

echo "==> building Crossplane from this branch"
# The image streams on stdout, so the build's own output has to go somewhere
# other than the pipe - but not to /dev/null. A failed Nix build then streams
# nothing, docker load says only "unrecognized image format", and the actual
# error is gone. The commonest cause is a stale Go vendor hash after a
# dependency change, which `nix run .#tidy` fixes and which says so plainly if
# you can see it.
build_log="$(mktemp)"
image="$(cd "${repo}" && nix run .#stream-image 2>"${build_log}" | docker load | sed -n 's/^Loaded image: //p')"
[ -n "${image}" ] || {
	echo "could not build the Crossplane image:" >&2
	tail -20 "${build_log}" >&2
	echo >&2
	echo "if that mentions a hash mismatch, run: nix run .#tidy" >&2
	exit 1
}
rm -f "${build_log}"
echo "    ${image}"
"${KIND[@]}" load docker-image "${image}" --name "${cluster}"

echo "==> switching Crossplane to that image, with ordering on"
args='"core","start","--debug","--enable-composed-resource-ordering"'
for a in ${extra_args}; do
	args="${args},\"${a}\""
done

kubectl -n crossplane-system patch deploy crossplane --type=json -p "[
 {\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/image\",\"value\":\"${image}\"},
 {\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/imagePullPolicy\",\"value\":\"Never\"},
 {\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/args\",\"value\":[${args}]}
]"
kubectl -n crossplane-system rollout status deploy/crossplane --timeout=5m

# Check every running pod, with a retry: just after a rollout the old pod can
# still be Running, and it is the one without the flag.
echo "==> confirming the feature flag took"
enabled=""
for _ in $(seq 1 30); do
	for pod in $(kubectl -n crossplane-system get pods -l app=crossplane \
		--field-selector=status.phase=Running -o name 2>/dev/null); do
		if kubectl -n crossplane-system logs "${pod}" -c crossplane 2>/dev/null |
			grep -q "Alpha feature enabled"; then
			enabled=yes
			break 2
		fi
	done
	sleep 2
done
[ -n "${enabled}" ] || {
	echo "!! the ordering feature flag did not take; check the deployment args" >&2
	exit 1
}

echo "==> installing the function, provider and XR API"
kubectl apply -f "${here}/manifests/packages.yaml"
kubectl apply -f "${here}/manifests/xrd.yaml"
kubectl wait --for=condition=Healthy --timeout=5m \
	function/function-ordering provider/provider-nop
kubectl wait --for=condition=Established --timeout=2m \
	xrd/xscales.scale.crossplane.io

echo "==> installing metrics-server, so runs can report what they used"
"${here}/metrics-server.sh"

echo
echo "Ready. Run ./run.py --help for a measurement."
