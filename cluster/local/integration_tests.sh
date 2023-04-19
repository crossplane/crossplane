#!/usr/bin/env bash
set -e

# setting up colors
BLU='\033[0;34m'
YLW='\033[0;33m'
GRN='\033[0;32m'
RED='\033[0;31m'
NOC='\033[0m' # No Color
echo_info() {
    printf "\n${BLU}%s${NOC}" "$1"
}
echo_step() {
    printf "\n${BLU}>>>>>>> %s${NOC}\n" "$1"
}
echo_sub_step() {
    printf "\n${BLU}>>> %s${NOC}\n" "$1"
}

echo_step_completed() {
    printf "${GRN} [✔]${NOC}"
}

echo_success() {
    printf "\n${GRN}%s${NOC}\n" "$1"
}
echo_warn() {
    printf "\n${YLW}%s${NOC}" "$1"
}
echo_error() {
    printf "\n${RED}%s${NOC}" "$1"
    exit 1
}

# ------------------------------
projectdir="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)"

# get the build environment variables from the special build.vars target in the main makefile
eval $(make --no-print-directory -C ${projectdir} build.vars)

SAFEHOSTARCH="${SAFEHOSTARCH:-amd64}"
BUILD_IMAGE="${BUILD_REGISTRY}/${PROJECT_NAME}-${SAFEHOSTARCH}"

helm_tag="$(cat ${projectdir}/_output/version)"
CROSSPLANE_IMAGE="${PROJECT_NAME}/${PROJECT_NAME}:${helm_tag}"
K8S_CLUSTER="${K8S_CLUSTER:-${BUILD_REGISTRY}-inttests}"

CROSSPLANE_NAMESPACE="crossplane-system"

# cleanup on exit
if [ "$skipcleanup" != true ]; then
    function cleanup() {
        echo_step "Cleaning up..."
        export KUBECONFIG=
        "${KIND}" delete cluster --name="${K8S_CLUSTER}"
    }

    trap cleanup EXIT
fi

readonly DEFAULT_KIND_CONFIG="kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
"
echo_step "creating k8s cluster using kind with the following config:"
kind_config="${KIND_CONFIG:-$DEFAULT_KIND_CONFIG}"
echo "${kind_config}"
echo "${kind_config}" | "${KIND}" create cluster --name="${K8S_CLUSTER}" --config=-

# tag crossplane image and load it to kind cluster
docker tag "${BUILD_IMAGE}" "${CROSSPLANE_IMAGE}"
"${KIND}" load docker-image "${CROSSPLANE_IMAGE}" --name="${K8S_CLUSTER}"

echo_step "installing helm package(s) into \"${CROSSPLANE_NAMESPACE}\" namespace"
"${HELM3}" install --create-namespace -n "${CROSSPLANE_NAMESPACE}" "${PROJECT_NAME}" "${projectdir}/cluster/charts/${PROJECT_NAME}" --set replicas=2,args={'-d'},rbacManager.replicas=2,rbacManager.args={'-d'},image.pullPolicy=Never,imagePullSecrets='',image.tag=${helm_tag}

echo_step "waiting for deployment ${PROJECT_NAME} rollout to finish"
"${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" rollout status "deploy/${PROJECT_NAME}" --timeout=2m

echo_step "wait until the pods are up and running"
"${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" wait --for=condition=Ready pods --all --timeout=1m

# ----------- integration tests
echo_step "------------------------------ INTEGRATION TESTS"
echo
echo_step "check for necessary deployment statuses"
echo
echo -------- deployments
"${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" get deployments

MUST_HAVE_DEPLOYMENTS="crossplane crossplane-rbac-manager"
for name in $MUST_HAVE_DEPLOYMENTS; do
    echo_sub_step "inspecting deployment '${name}'"
    dep_stat=$("${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" get deployments/"${name}")

    echo_info "check if is deployed"
    if $(echo "$dep_stat" | grep -iq 'No resources found'); then
        echo "is not deployed"
        exit -1
    else
        echo_step_completed
    fi

    echo_info "check if is ready"
    IFS='/' read -ra ready_status_parts <<<"$(echo "$dep_stat" | awk ' FNR > 1 {print $2}')"
    if ((${ready_status_parts[0]} < ${ready_status_parts[1]})); then
        echo "is not Ready"
        exit -1
    else
        echo_step_completed
    fi
    echo
done

echo_step "check for pods statuses"
for ((i = 1; i <= 5; i++)); do
    echo_sub_step "pod check #$i"
    echo
    echo "-------- pods"
    pods=$("${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" get pods)
    echo "$pods"
    while read -r pod_stat; do
        name=$(echo "$pod_stat" | awk '{print $1}')
        echo_sub_step "inspecting pod '${name}'"

        echo_info "check if is ready"
        IFS='/' read -ra ready_status_parts <<<"$(echo "$pod_stat" | awk '{print $2}')"
        if ((${ready_status_parts[0]} < ${ready_status_parts[1]})); then
            echo_error "is not ready"
            exit -1
        else
            echo_step_completed
        fi

        echo_info "check if is running"
        if $(echo "$pod_stat" | awk '{print $3}' | grep -ivq 'Running'); then
            echo_error "is not running"
            exit -1
        else
            echo_step_completed
        fi

        echo_info "check if has error in logs"
        error_logs=$("${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" logs --all-containers=true --timestamps=true --tail=10 "${name}" | grep -w ERROR || true)
        if ((${#error_logs} > 0)); then
            echo_warn "${error_logs}"
        else
            echo_step_completed
        fi

        echo_info "check if has restarts"
        if (($(echo "$pod_stat" | awk '{print $4}') > 0)); then
            echo_error "has restarts"
            exit -1
        else
            echo_step_completed
        fi
        echo
    done <<<"$(echo "$pods" | awk 'FNR>1')"
    sleep 5
done

echo_success "Integration tests succeeded!"
