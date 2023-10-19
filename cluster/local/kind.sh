#!/usr/bin/env bash

set -e

scriptdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# shellcheck disable=SC1090
projectdir="${scriptdir}/../.."

# get the build environment variables from the special build.vars target in the main makefile
eval $(make --no-print-directory -C ${scriptdir}/../.. build.vars)

# ensure the tools we need are installed
make ${KIND} ${KUBECTL} ${HELM3}

BUILD_IMAGE="${BUILD_REGISTRY}/${PROJECT_NAME}-${TARGETARCH}"
DEFAULT_NAMESPACE="crossplane-system"

function copy_image_to_cluster() {
    local build_image=$1
    local final_image=$2
    local kind_name=$3
    docker tag "${build_image}" "${final_image}"
    ${KIND} --name "${kind_name}" load docker-image "${final_image}"
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
KIND_NAME=${KIND_NAME:-"kind"}
IMAGE_REPOSITORY="xpkg.upbound.io/${PROJECT_NAME}/${PROJECT_NAME}"
case "${1:-}" in
  up)
    ${KIND} create cluster --name "${KIND_NAME}" --image "${KUBE_IMAGE}" --wait 5m
    ;;
  update)
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} "${IMAGE_REPOSITORY}:${helm_tag}" "${KIND_NAME}"
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
    copy_image_to_cluster ${BUILD_IMAGE} "${IMAGE_REPOSITORY}:${helm_tag}" "${KIND_NAME}"

    [ "$2" ] && ns=$2 || ns="${DEFAULT_NAMESPACE}"
    echo "installing helm package into \"$ns\" namespace"
    ${HELM3} install ${PROJECT_NAME} --namespace ${ns} --create-namespace ${projectdir}/cluster/charts/${PROJECT_NAME} --set image.pullPolicy=Never,imagePullSecrets='',image.tag="${helm_tag}" --set args='{"--debug"}' ${HELM3_FLAGS}
    ;;
  helm-upgrade)
    echo "copying image for helm"
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} "${IMAGE_REPOSITORY}:${helm_tag}" "${KIND_NAME}"

    [ "$2" ] && ns=$2 || ns="${DEFAULT_NAMESPACE}"
    echo "upgrading helm package in \"$ns\" namespace"
    ${HELM3} upgrade --install --namespace ${ns} --create-namespace ${PROJECT_NAME} ${projectdir}/cluster/charts/${PROJECT_NAME} --set image.pullPolicy=Never,imagePullSecrets='',image.tag=${helm_tag} --set args='{"--debug"}' ${HELM3_FLAGS}
    ;;
  helm-delete)
    [ "$2" ] && ns=$2 || ns="${DEFAULT_NAMESPACE}"
    echo "removing helm package from \"$ns\" namespace"
    ${HELM3} uninstall --namespace ${ns} ${PROJECT_NAME}
    ;;
  helm-list)
    ${HELM3} list --all --all-namespaces
    ;;
  clean)
    ${KIND} --name "${KIND_NAME}" delete cluster
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
