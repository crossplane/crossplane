#!/usr/bin/env bash
set -aeuo pipefail

COMPONENT=$1

source "${SCRIPTS_DIR}/utils.sh"
source "${SCRIPTS_DIR}/load-configs.sh" "${COMPONENT}"

DEPLOY_SCRIPT="${DEPLOY_LOCAL_CONFIG_DIR}/${COMPONENT}/deploy.sh"

# Run deploy script, if exists.
# If there is a deploy.sh script, which indicates this is a "script-only" component, removing not supported.
if [ -f "${DEPLOY_SCRIPT}" ]; then
  echo_warn "${COMPONENT} is a \"script-only\" component, local.remove not supported!"
  exit 0
fi


if [ -z "${HELM_RELEASE_NAME}" ]; then
  HELM_RELEASE_NAME=${COMPONENT}
fi

helm_purge_flag="--purge"
if [ "${USE_HELM3}" == "true" ]; then
  HELM="${HELM3}"
  XDG_DATA_HOME="${HELM_HOME}"
  XDG_CONFIG_HOME="${HELM_HOME}"
  XDG_CACHE_HOME="${HELM_HOME}"
  helm_purge_flag=""
fi

echo_info "Running helm delete..."
set -x
"${HELM}" delete "${HELM_RELEASE_NAME}" -n "${HELM_RELEASE_NAMESPACE}" --kubeconfig "${KUBECONFIG}" ${helm_purge_flag}
{ set +x; } 2>/dev/null
echo_info "Running helm delete...OK!"