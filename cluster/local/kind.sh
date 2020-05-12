#!/usr/bin/env bash

set -e

scriptdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# shellcheck disable=SC1090
projectdir="${scriptdir}/../.."

# get the build environment variables from the special build.vars target in the main makefile
eval $(make --no-print-directory -C ${scriptdir}/../.. build.vars)

BUILD_IMAGE="${BUILD_REGISTRY}/${PROJECT_NAME}-amd64"
DEPLOYMENT_IMAGE="${DOCKER_REGISTRY}/${PROJECT_NAME}:master"
DEFAULT_NAMESPACE="crossplane-system"

function copy_image_to_cluster() {
    local build_image=$1
    local final_image=$2
    docker tag "${build_image}" "${final_image}"
    kind load docker-image "${final_image}"
    echo "Tagged image: ${final_image}"
}

# Deletes pods with application prefix. Namespace is expected as the first argument
function delete_pods() {
    for pod in $(kubectl get pods -n "$2" -l "app=$1" --no-headers -o custom-columns=NAME:.metadata.name); do
        kubectl delete pod "$pod" -n "$2"
    done
}

# current kubectl context == kind-kind, returns boolean
function check_context() {
    if [ "$(kubectl config view 2>/dev/null | awk '/current-context/ {print $NF}')" = "kind-kind" ]; then
        return 0
    fi

    return 1
}

# configure kind
KUBE_IMAGE=${KUBE_IMAGE:-"kindest/node:v1.15.11@sha256:6cc31f3533deb138792db2c7d1ffc36f7456a06f1db5556ad3b6927641016f50"}

case "${1:-}" in
  up)
    kind create cluster --image "${KUBE_IMAGE}" --wait 5m

    kubectl apply -f ${scriptdir}/helm-rbac.yaml
    ${HELM} init --service-account tiller
    kubectl -n kube-system rollout status deploy/tiller-deploy

    copy_image_to_cluster ${BUILD_IMAGE} ${DEPLOYMENT_IMAGE}
    ;;
  update)
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} ${DEPLOYMENT_IMAGE}
    copy_image_to_cluster ${BUILD_IMAGE} "${DOCKER_REGISTRY}/${PROJECT_NAME}:${helm_tag}"
    ;;
  restart)
    if check_context; then
        [ "$2" ] && ns=$2 || ns="${DEFAULT_NAMESPACE}"
        echo "Restarting \"${PROJECT_NAME}\" deployment pods in \"$ns\" namespace."
        delete_pods ${PROJECT_NAME} ${ns}
    else
      echo "To prevent accidental data loss acting only on 'kind-kind' context. No action is taken."
    fi
    ;;
  helm-install)
    echo "copying image for helm"
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} "${DOCKER_REGISTRY}/${PROJECT_NAME}:${helm_tag}"

    [ "$2" ] && ns=$2 || ns="${DEFAULT_NAMESPACE}"
    echo "installing helm package(s) into \"$ns\" namespace"
    ${HELM} install --name ${PROJECT_NAME} --namespace ${ns} ${projectdir}/cluster/charts/${PROJECT_NAME} --set image.pullPolicy=Never,imagePullSecrets=''
    ;;
  helm-upgrade)
    echo "copying image for helm"
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} "${DOCKER_REGISTRY}/${PROJECT_NAME}:${helm_tag}"
    ${HELM} upgrade ${PROJECT_NAME} ${projectdir}/cluster/charts/${PROJECT_NAME}
    ;;
  helm-delete)
    echo "removing helm package"
    ${HELM} del --purge ${PROJECT_NAME}
    ;;
  helm-list)
    ${HELM} list ${PROJECT_NAME} --all
    ;;
  clean)
    kind delete cluster
    ;;
  *)
    echo "usage:" >&2
    echo "  $0 up - create a new kind cluster" >&2
    echo "  $0 clean - delete the kind cluster" >&2
    echo "  $0 update - push project docker images to kind cluster registry" >&2
    echo "  $0 restart project deployment pod(s) in specified namespace [default: \"${DEFAULT_NAMESPACE}\"]" >&2
    echo "  $0 helm-install package(s) into provided namespace [default: \"${DEFAULT_NAMESPACE}\"]" >&2
    echo "  $0 helm-upgrade - deploy the latest docker images and helm charts to kind cluster" >&2
    echo "  $0 helm-delete package(s)" >&2
    echo "  $0 helm-list all package(s)" >&2
esac
