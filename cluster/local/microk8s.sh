#!/usr/bin/env bash

set -e

scriptdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# shellcheck disable=SC1090
projectdir="${scriptdir}/../.."

# get the build environment variables from the special build.vars target in the main makefile
eval $(make --no-print-directory -C ${scriptdir}/../.. build.vars)

MICROK8S_REGISTRY="localhost:32000"
BUILD_IMAGE="${BUILD_REGISTRY}/${PROJECT_NAME}-amd64"
# Use microk8s registry add-on
MICROK8S_IMAGE="${MICROK8S_REGISTRY}/${PROJECT_NAME}:master"
DEFAULT_NAMESPACE="crossplane-system"

function wait_for_ready() {
    local tries=100
    while (( tries > 0 )) ; do
        if nc -z localhost 32000 ; then
          return 0
        fi
        echo "Waiting for registry..."
        tries=$(( tries - 1 ))
        sleep 1
    done
    echo ERROR: registry did not become available >&2
    exit 1
}

function copy_image_to_cluster() {
    local build_image=$1
    local final_image=$2
    docker tag "${build_image}" "${final_image}" && docker push "${final_image}"
    echo "Tagged image: ${final_image}"
}

# Deletes pods with application prefix. Namespace is expected as the first argument
function delete_pods() {
    for pod in $(kubectl get pods -n "$2" -l "app=$1" --no-headers -o custom-columns=NAME:.metadata.name); do
        kubectl delete pod "$pod" -n "$2"
    done
}

case "${1:-}" in
  up)
    microk8s.start

    microk8s.enable ingress

    microk8s.enable registry

    kubectl apply -f ${scriptdir}/helm-rbac.yaml
    ${HELM} init --service-account tiller

    wait_for_ready

    copy_image_to_cluster ${BUILD_IMAGE} ${MICROK8S_IMAGE}
    ;;
  down)
    microk8s.stop
    ;;
  update)
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} ${MICROK8S_IMAGE}
    copy_image_to_cluster ${BUILD_IMAGE} "${MICROK8S_REGISTRY}/${PROJECT_NAME}:${helm_tag}"
    ;;
  restart)
    ns="${DEFAULT_NAMESPACE}"
    echo "Restarting \"${PROJECT_NAME}\" deployment pods in \"$ns\" namespace."
    delete_pods ${PROJECT_NAME} ${ns}
    ;;
  helm-install)
    echo "copying image for helm"
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} "${MICROK8S_REGISTRY}/${PROJECT_NAME}:${helm_tag}"

    [ "$2" ] && ns=$2 || ns="${DEFAULT_NAMESPACE}"
    echo "installing helm package(s) into \"$ns\" namespace"
    ${HELM} install --name ${PROJECT_NAME} --namespace ${ns} ${projectdir}/cluster/charts/${PROJECT_NAME} --set image.pullPolicy=Always,imagePullSecrets='',image.repository='localhost:32000/crossplane'
    ;;
  helm-upgrade)
    echo "copying image for helm"
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} "${MICROK8S_REGISTRY}/${PROJECT_NAME}:${helm_tag}"
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
    microk8s.reset
    ;;
  *)
    echo "usage:" >&2
    echo "  $0 up - start new instance of Kubernetes cluster in microk8s" >&2
    echo "  $0 down - stop Kubernetes cluster in microk8s" >&2
    echo "  $0 clean - reset microk8s" >&2
    echo "  $0 update - push project docker images to microk8s" >&2
    echo "  $0 restart project deployment pod(s) in specified namespace [default: \"${DEFAULT_NAMESPACE}\"]" >&2
    echo "  $0 helm-install package(s) into provided namespace [default: \"${DEFAULT_NAMESPACE}\"]" >&2
    echo "  $0 helm-upgrade - deploy the latest docker images and helm charts to microk8s" >&2
    echo "  $0 helm-delete package(s)" >&2
    echo "  $0 helm-list all package(s)" >&2
esac
