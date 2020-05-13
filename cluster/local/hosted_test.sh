#!/usr/bin/env bash
set -euo pipefail

scriptdir="$( dirname "${BASH_SOURCE[0]}")"
projectdir="$( cd "${scriptdir}"/../.. && pwd )"

eval $(make --no-print-directory -C "${projectdir}" build.vars)

TAG=$(cat "${projectdir}"/_output/version)
BUILD_IMAGE="${BUILD_REGISTRY}/${PROJECT_NAME}-amd64"
FINAL_IMAGE="${DOCKER_REGISTRY}/${PROJECT_NAME}:${TAG}"

types_chart_dir="${projectdir}/cluster/charts/crossplane-types"
controllers_chart_dir="${projectdir}/cluster/charts/crossplane-controllers"

kind_context=local-hosted-test
pseudo_tenant_namespace=default
system_namespace=crossplane-hosted-system
controllers_namespace=crossplane-hosted-controllers
tenant_kubeconfig_secret=tenant-kubeconfig

tenant_kubernetes_service_host=kubernetes.default.svc
tenant_kubernetes_service_port=443

echo "creating kind cluster ${kind_context} - if not exists"
if [[ -z "${KUBECONFIG:-}" ]]; then
    export KUBECONFIG="$HOME/.kube/config"
fi
kind get kubeconfig --name ${kind_context} > /dev/null 2>&1 || \
  kind create cluster --name="${kind_context}" --kubeconfig=${KUBECONFIG}

echo "creating hosted system and controllers namespaces"
kubectl create ns "${system_namespace}" -o yaml --dry-run | kubectl apply -f -
kubectl create ns "${controllers_namespace}" -o yaml --dry-run | kubectl apply -f -

echo "creating kubeconfig secret to access cluster from inside"
local_kubeconfig=$(sed -e 's|server:\s*.*$|server: https://kubernetes.default.svc|g' "${KUBECONFIG}")
kubectl -n "${system_namespace}" create secret generic "${tenant_kubeconfig_secret}" --from-literal=kubeconfig="${local_kubeconfig}" -o yaml --dry-run \
  | kubectl apply -f -

echo "tagging crossplane image and loading into kind cluster"
docker tag "${BUILD_IMAGE}" "${FINAL_IMAGE}"
kind load docker-image "${FINAL_IMAGE}" --name="${kind_context}"

echo "deploying crossplane-types"
helm3 upgrade --install cp-types -n "${pseudo_tenant_namespace}" "${types_chart_dir}"


crossplane_sa_token_secret=""
until [[ -n "${crossplane_sa_token_secret}" ]]; do
    echo "wait until crossplane service account token secret generated"
    crossplane_sa_token_secret=$(kubectl -n "${pseudo_tenant_namespace}" get sa crossplane -o jsonpath="{.secrets[0].name}" || true)
    sleep 3
done

echo "copying service account token from (pseudo) tenant to host"
token=$(kubectl get secret -n "${pseudo_tenant_namespace}" "${crossplane_sa_token_secret}" -o jsonpath="{.data.token}" | base64 --decode)
ca=$(kubectl get secret -n "${pseudo_tenant_namespace}" "${crossplane_sa_token_secret}" -o jsonpath="{.data.ca\.crt}" | base64 --decode)
ns=$(kubectl get secret -n "${pseudo_tenant_namespace}" "${crossplane_sa_token_secret}" -o jsonpath="{.data.namespace}" | base64 --decode)
kubectl  -n "${system_namespace}" create secret generic "${crossplane_sa_token_secret}" \
  --from-literal=token="${token}" \
  --from-literal=ca.crt="${ca}" \
  --from-literal=namespace="${ns}" \
  -o yaml --dry-run |kubectl apply -f -

echo "deploying crossplane-controllers"
helm3 upgrade --install cp-controllers -n "${system_namespace}" "${controllers_chart_dir}" \
  --set image.pullPolicy=Never \
  --set hostedConfig.enabled=true \
  --set hostedConfig.controllerNamespace="${controllers_namespace}" \
  --set hostedConfig.tenantKubeconfigSecret="${tenant_kubeconfig_secret}" \
  --set hostedConfig.tenantKubernetesServiceHost="${tenant_kubernetes_service_host}" \
  --set hostedConfig.tenantKubernetesServicePort="${tenant_kubernetes_service_port}" \
  --set hostedConfig.crossplaneSATokenSecret="${crossplane_sa_token_secret}"

echo """
Kind cluster is ${kind_context} created is created:

  export KUBECONFIG=${KUBECONFIG}

A very basic check is to deploy a package:

  kubectl crossplane package generate-install --cluster 'crossplane/provider-gcp:master' provider-gcp | kubectl apply -f -

and make sure that its controllers is running in ${controllers_namespace} namespace.
"""
