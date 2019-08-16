#!/usr/bin/env bash

set -e

scriptdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# shellcheck disable=SC1090
projectdir="${scriptdir}/../.."

# get the build environment variables from the special build.vars target in the main makefile
eval $(make --no-print-directory -C ${scriptdir}/../.. build.vars)

BUILD_IMAGE="${BUILD_REGISTRY}/${PROJECT_NAME}-amd64"
MINIKUBE_IMAGE="${DOCKER_REGISTRY}/${PROJECT_NAME}:master"
DEFAULT_NAMESPACE="crossplane-system"

function wait_for_ssh() {
    local tries=100
    while (( tries > 0 )) ; do
        if minikube ssh echo connected &> /dev/null ; then
            return 0
        fi
        tries=$(( tries - 1 ))
        sleep 0.1
    done
    echo ERROR: ssh did not come up >&2
    exit 1
}

function copy_image_to_cluster() {
    local build_image=$1
    local final_image=$2
    docker save "${build_image}" | (eval "$(minikube docker-env --shell bash)" && docker load && docker tag "${build_image}" "${final_image}")
    echo "Tagged image: ${final_image}"
}

# Deletes pods with application prefix. Namespace is expected as the first argument
function delete_pods() {
    for pod in $(kubectl get pods -n "$2" -l "app=$1" --no-headers -o custom-columns=NAME:.metadata.name); do
        kubectl delete pod "$pod" -n "$2"
    done
}

# current kubectl context == minikube, returns boolean
function check_context() {
    if [ "$(kubectl config view 2>/dev/null | awk '/current-context/ {print $NF}')" = "minikube" ]; then
        return 0
    fi

    return 1
}

# configure minikube
KUBE_VERSION=${KUBE_VERSION:-"v1.15.2"}
MEMORY=${MEMORY:-"3000"}
DRIVER=${DRIVER:-virtualbox}

case "${1:-}" in
  up)
    minikube start --vm-driver ${DRIVER} --memory ${MEMORY} --kubernetes-version ${KUBE_VERSION}

    wait_for_ssh

    minikube addons enable ingress

    kubectl apply -f ${scriptdir}/helm-rbac.yaml
    ${HELM} init --service-account tiller
    kubectl -n kube-system rollout status deploy/tiller-deploy
    kubectl -n kube-system rollout status deploy/nginx-ingress-controller

    copy_image_to_cluster ${BUILD_IMAGE} ${MINIKUBE_IMAGE}
    ;;
  down)
    minikube stop
    ;;
  ssh)
    echo "connecting to minikube"
    minikube ssh
    ;;
  update)
    helm_tag="$(cat _output/version)"
    copy_image_to_cluster ${BUILD_IMAGE} ${MINIKUBE_IMAGE}
    copy_image_to_cluster ${BUILD_IMAGE} "${DOCKER_REGISTRY}/${PROJECT_NAME}:${helm_tag}"
    ;;
  restart)
    if check_context; then
        [ "$2" ] && ns=$2 || ns="${DEFAULT_NAMESPACE}"
        echo "Restarting \"${PROJECT_NAME}\" deployment pods in \"$ns\" namespace."
        delete_pods ${PROJECT_NAME} ${ns}
    else
      echo "To prevent accidental data loss acting only on 'minikube' context. No action is taken."
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
    minikube delete
    ;;
  *)
    echo "usage:" >&2
    echo "  $0 up - start new instance of Kubernetes cluster in minikube vm" >&2
    echo "  $0 down - stop Kubernetes cluster in minikube vm" >&2
    echo "  $0 clean - delete minikube vm" >&2
    echo "  $0 ssh - open ssh connection to minikube vm" >&2
    echo "  $0 update - push project docker images to minikube docker" >&2
    echo "  $0 restart project deployment pod(s) in specified namespace [default: \"${DEFAULT_NAMESPACE}\"]" >&2
    echo "  $0 helm-install package(s) into provided namespace [default: \"${DEFAULT_NAMESPACE}\"]" >&2
    echo "  $0 helm-upgrade - deploy the latest docker images and helm charts to minikube" >&2
    echo "  $0 helm-delete package(s)" >&2
    echo "  $0 helm-list all package(s)" >&2
esac
