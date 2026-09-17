#!/usr/bin/env bash
# Stand up a throwaway kind cluster running Crossplane from this branch, with
# composed resource ordering enabled and the demo's packages installed.
#
# Takes a few minutes the first time, mostly pulling images. Re-runnable: it
# deletes and recreates the cluster.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
# demo/ sits four levels down, under design/assets/<doc>/.
repo="$(cd "${here}/../../../.." && pwd)"
cluster="${CLUSTER:-xp-ordering-demo}"
arch="$(go env GOARCH)"

echo "==> creating kind cluster ${cluster}"
kind delete cluster --name "${cluster}" >/dev/null 2>&1 || true
kind create cluster --name "${cluster}"

echo "==> installing a released Crossplane, for its CRDs, RBAC and TLS secrets"
kubectl create namespace crossplane-system
helm install crossplane "${repo}/cluster/charts/crossplane" \
  -n crossplane-system \
  --set image.repository=crossplane/crossplane --set image.tag=v2.4.0 \
  --wait

echo "==> building crossplane from this branch (${arch})"
# A directory of its own, not /tmp: the build context is sent to the daemon in
# full, and anything unreadable in a shared /tmp fails the build.
ctx="$(mktemp -d)"
trap 'rm -rf "${ctx}"' EXIT

CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" \
  go build -o "${ctx}/crossplane" "${repo}/cmd/crossplane"

cat > "${ctx}/Dockerfile" <<'DOCKERFILE'
FROM gcr.io/distroless/static-debian12:nonroot
COPY crossplane /usr/local/bin/crossplane
USER 65532
ENTRYPOINT ["crossplane"]
DOCKERFILE
docker build -q -t xp-ordering/crossplane:demo "${ctx}"
kind load docker-image xp-ordering/crossplane:demo --name "${cluster}"

echo "==> switching Crossplane to that image, with ordering on"
# --circuit-breaker-burst is raised well above its default of 100. Realtime
# compositions trip a per-XR breaker when an XR sees a burst of watch events,
# after which events are only let through every 30s - correct behavior for a
# runaway XR, but a four-level graph trips it and each later wave then takes
# about a minute. That is death for a live demo. Raising the burst keeps the
# breaker out of the way; it changes nothing about ordering itself.
kubectl -n crossplane-system patch deploy crossplane --type=json -p '[
 {"op":"replace","path":"/spec/template/spec/containers/0/image","value":"xp-ordering/crossplane:demo"},
 {"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Never"},
 {"op":"replace","path":"/spec/template/spec/containers/0/args","value":[
   "core","start","--debug",
   "--enable-composed-resource-ordering",
   "--circuit-breaker-burst=100000"
 ]}
]'
kubectl -n crossplane-system rollout status deploy/crossplane --timeout=3m

# Check every running pod, with a retry. Just after a rollout the old pod can
# still be Running - and it is the one without the flag - so asking the
# deployment for "the" pod can read the wrong one and log lag can hide the
# right one.
echo "==> confirming the feature flag took"
enabled=""
for _ in $(seq 1 30); do
  for pod in $(kubectl -n crossplane-system get pods -l app=crossplane \
      --field-selector=status.phase=Running -o name 2>/dev/null); do
    if kubectl -n crossplane-system logs "${pod}" -c crossplane 2>/dev/null \
        | grep -q "Alpha feature enabled"; then
      enabled=yes
      break 2
    fi
  done
  sleep 2
done

if [ -z "${enabled}" ]; then
  echo "!! the ordering feature flag did not take; check the deployment args" >&2
  kubectl -n crossplane-system get deploy crossplane \
    -o jsonpath='{.spec.template.spec.containers[0].args}{"\n"}' >&2
  exit 1
fi
echo "==> ordering is enabled"

echo "==> installing the function, provider and XR API"
kubectl apply -f "${here}/manifests/00-packages.yaml"
kubectl apply -f "${here}/manifests/00-xrd.yaml"
kubectl wait --for=condition=Healthy --timeout=5m \
  function/function-ordering provider/provider-nop
kubectl wait --for=condition=Established --timeout=2m \
  xrd/xnetworks.demo.crossplane.io

echo
echo "Ready. Run ./watch.sh in a second pane, then follow README.md."
