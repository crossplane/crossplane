COMPONENT=$1

# REQUIRED_IMAGES is the array of images that the COMPONENT needs.
# These images will be pulled (if not exists) and loaded into the kind cluster before deployment.
# If an image has tags, it will be used.
# If an image does not have a tag, "v${HELM_CHART_VERSION}" will be used as a tag.
REQUIRED_IMAGES=()

# HELM_CHART_NAME is the name of the helm chart to deploy. If not set, defaults to COMPONENT
HELM_CHART_NAME=""
# HELM_CHART_VERSION is the version of the helm chart to deploy.
# If LOCALDEV_LOCAL_BUILD=true, HELM_CHART_VERSION will be set the version in build system.
# If LOCALDEV_LOCAL_BUILD=false, HELM_CHART_VERSION defaults to latest version in the HELM_REPOSITORY
HELM_CHART_VERSION=""
# HELM_REPOSITORY_NAME is the name of the helm repository.
# This will only be used if LOCALDEV_LOCAL_BUILD=false or HELM_CHART_NAME is not a local chart (e.g. not in HELM_CHARTS array)
HELM_REPOSITORY_NAME=""
# HELM_REPOSITORY_NAME is the url of the helm repository.
HELM_REPOSITORY_URL=""
# HELM_REPOSITORY_FORCE_UPDATE controls whether always update helm repositories or not.
# If false, "helm repo update" will only be called if repo does not exist already.
HELM_REPOSITORY_FORCE_UPDATE="false"
# HELM_RELEASE_NAME is the name of the helm release. If not set, defaults to COMPONENT
HELM_RELEASE_NAME=""
# HELM_RELEASE_NAMESPACE is the namespace for the helm release.
HELM_RELEASE_NAMESPACE="default"
# HELM_DELETE_ON_FAILURE controls whether to delete/rollback a failed install/upgrade.
HELM_DELETE_ON_FAILURE="true"

# COMPONENT_SKIP_DEPLOY controls whether (conditionally) skip deployment of a component or not.
COMPONENT_SKIP_DEPLOY="false"

MAIN_CONFIG_FILE="${DEPLOY_LOCAL_CONFIG_DIR}/config.env"
COMPONENT_CONFIG_DIR="${DEPLOY_LOCAL_CONFIG_DIR}/${COMPONENT}"
COMPONENT_CONFIG_FILE="${COMPONENT_CONFIG_DIR}/config.env"

if [[ ! -d "${COMPONENT_CONFIG_DIR}" ]]; then
  echo_error "Component config dir \"${COMPONENT_CONFIG_DIR}\" does not exist (or is not a directory), did you run make local.prepare ?"
fi

if [[ -f "${MAIN_CONFIG_FILE}" ]]; then
  source "${MAIN_CONFIG_FILE}"
fi
if [[ -f "${COMPONENT_CONFIG_FILE}" ]]; then
  source "${COMPONENT_CONFIG_FILE}"
fi
