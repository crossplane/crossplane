#!/usr/bin/env bash
set -e

projectdir="$( cd "$( dirname "${BASH_SOURCE[0]}")"/../.. && pwd )"

# get the build environment variables from the special build.vars target in the main makefile
eval $(make --no-print-directory -C ${projectdir} build.vars)

HOSTARCH="${HOSTARCH:-amd64}"
BUILD_IMAGE="${BUILD_REGISTRY}/${PROJECT_NAME}-${HOSTARCH}"

helm_tag="$(cat ${projectdir}/_output/version)"
CROSSPLANE_IMAGE="${DOCKER_REGISTRY}/${PROJECT_NAME}:${helm_tag}"
K8S_CLUSTER="${BUILD_REGISTRY}-INTTESTS"

CROSSPLANE_NAMESPACE="crossplane-system"

# cleanup on exit
function cleanup {
    export KUBECONFIG=
    "${KIND}" delete cluster --name="${K8S_CLUSTER}"
}
trap cleanup EXIT

# create cluster
"${KIND}" create cluster --name="${K8S_CLUSTER}"
export KUBECONFIG="$("${KIND}" get kubeconfig-path --name="${K8S_CLUSTER}")"

# tag crossplane image and load it to kind cluster
docker tag "${BUILD_IMAGE}" "${CROSSPLANE_IMAGE}"
"${KIND}" load docker-image "${CROSSPLANE_IMAGE}" --name="${K8S_CLUSTER}"

# install tiller
"${KUBECTL}" apply -f "${projectdir}/cluster/local/helm-rbac.yaml"
"${HELM}" init --service-account tiller
# waiting for deployment "tiller-deploy" rollout to finish
"${KUBECTL}" -n kube-system rollout status deploy/tiller-deploy --timeout=2m

# install crossplane
echo "installing helm package(s) into \"${CROSSPLANE_NAMESPACE}\" namespace"
"${HELM}" install --name "${PROJECT_NAME}" --namespace "${CROSSPLANE_NAMESPACE}" "${projectdir}/cluster/charts/${PROJECT_NAME}" --set image.pullPolicy=Never,imagePullSecrets=''

# waiting for deployment "crossplane" rollout to finish
"${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" rollout status "deploy/${PROJECT_NAME}" --timeout=2m
echo "wait for 5 seconds so that the pods are up and running"
sleep 5

# ----------- integration tests
# get the pods statuses
pods_statuses=$("${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" get pods -o=jsonpath='{range .items[*]}{@.metadata.name}{" is "}{@.status.phase}{"\n"}')

# check for minimum number of pods created
MIN_CROSSPLANE_PODS=2
PODS_COUNT=$(echo "$pods_statuses" | wc -l | tr -d ' ')
if (( ${PODS_COUNT} < ${MIN_CROSSPLANE_PODS} )); then
    echo "number of created pods are ${PODS_COUNT}, which is less than the minimum of ${MIN_CROSSPLANE_PODS}"
    exit -1
fi

# check for all pods to be running
echo "Crossplane pods:"
echo "$pods_statuses"
echo

PODS_NOT_RUNNING_COUNT=$(echo "$pods_statuses" | grep -iv 'is running' | wc -l | tr -d ' ')
if (( ${PODS_NOT_RUNNING_COUNT} > 0 )); then
    echo "${PODS_NOT_RUNNING_COUNT} of ${PODS_COUNT} pods are not running"
    exit -1
fi
