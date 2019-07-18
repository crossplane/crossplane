#!/usr/bin/env bash

set -e

scriptdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# shellcheck disable=SC1090
projectdir="${scriptdir}/../.."

# get the build environment variables from the special build.vars target in the main makefile
eval $(make --no-print-directory -C ${scriptdir}/../.. build.vars)

BUILD_IMAGE="${BUILD_REGISTRY}/${PROJECT_NAME}-amd64"
FINAL_IMAGE="${DOCKER_REGISTRY}/${PROJECT_NAME}:master"
DEFAULT_NAMESPACE="crossplane-system"

function copy_image_to_cluster() {
    local build_image=$1
    local final_image=$2
    docker tag "${build_image}" "${final_image}"
    echo "Tagged image: ${final_image}"
}

case "${1:-}" in
  up)
    kubectl apply -f ${scriptdir}/helm-rbac.yaml
    ${HELM} init --service-account tiller
    kubectl -n kube-system rollout status deploy/tiller-deploy
    copy_image_to_cluster ${BUILD_IMAGE} ${FINAL_IMAGE}
    ;;
  down)
    ;;
  update)
    copy_image_to_cluster ${BUILD_IMAGE} ${FINAL_IMAGE}
    ;;
  helm-install)
    echo " copying image for helm"
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
  *)
    echo "usage:" >&2
    echo "  $0 up - initialize the Kubernetes cluster for local deployment" >&2
    echo "  $0 down - deinitialize the Kubernetes cluster for local deployment" >&2
    echo "  $0 update - push project docker images to Kubernetes cluster" >&2
    echo "  $0 helm-install package(s) into provided namespace [default: \"${DEFAULT_NAMESPACE}\"]" >&2
    echo "  $0 helm-upgrade - deploy the latest docker images and helm charts" >&2
    echo "  $0 helm-delete package(s)" >&2
    echo "  $0 helm-list all package(s)" >&2
esac
