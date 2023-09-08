#!/usr/bin/env bash
set -aeuo pipefail

deploy_local_root=${ROOT_DIR}/cluster/local

local_config_root=${deploy_local_root}
test -d ${local_config_root}/config && { echo "Directory \"${local_config_root}/config\" already exists!"; exit 1; }

charts_arr=($BUILD_HELM_CHARTS_LIST)
default_component="${charts_arr[0]}"
read -p "Enter a name for component to deploy [${default_component}]: " component
component=${component:-"${default_component}"}

local_config_dir=${local_config_root}/config

echo "creating directory ${local_config_root}"
mkdir -p "${local_config_root}"
echo "creating directory ${local_config_dir}/${component}"
mkdir -p "${local_config_dir}/${component}"

echo "initiazing file ${local_config_root}/kind.yaml"
cat << EOF > ${local_config_root}/kind.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
EOF

echo "initiazing file ${local_config_dir}/config.env"
cat << EOF > ${local_config_dir}/config.env
IMAGE_CROSSPLANE="crossplane/crossplane"

echo "replace this with top level config"
PARAM_FROM_TOP_LEVEL_CONFIG="top-level-config"
EOF

echo "initiazing file ${local_config_dir}/config.validate.sh"
cat << EOF > ${local_config_dir}/config.validate.sh
echo "replace this with top level config validation script"
EOF

echo "initiazing file ${local_config_dir}/${component}/config.env"
cat << EOF > ${local_config_dir}/${component}/config.env
REQUIRED_IMAGES+=("\${IMAGE_CROSSPLANE}")

#HELM_CHART_NAME=""
#HELM_CHART_VERSION=""
#HELM_REPOSITORY_NAME=""
#HELM_REPOSITORY_URL=""
#HELM_REPOSITORY_FORCE_UPDATE="false"
#HELM_RELEASE_NAME=""
#HELM_RELEASE_NAMESPACE="default"

echo "replace this with component config"
PARAM_FROM_COMPONENT_CONFIG="component-config"
EOF

echo "initiazing file ${local_config_dir}/${component}/pre-deploy.sh"
cat << EOF > ${local_config_dir}/${component}/pre-deploy.sh
# remove this file if the component does not need pre-deploy steps.
echo "running pre-deploy script..."
EOF

echo "initiazing file ${local_config_dir}/${component}/post-deploy.sh"
cat << EOF > ${local_config_dir}/${component}/post-deploy.sh
# remove this file if the component does not need post-deploy steps.
echo "running post-deploy script..."
EOF

echo "initiazing file ${local_config_dir}/${component}/.gitignore"
cat << EOF > ${local_config_dir}/${component}/.gitignore
value-overrides.yaml
EOF

echo "initiazing file ${local_config_dir}/${component}/value-overrides.yaml.tmpl"
cat << EOF > ${local_config_dir}/${component}/value-overrides.yaml.tmpl
image:
  pullPolicy: Never

paramFromTopLevel: {{ .Env.PARAM_FROM_TOP_LEVEL_CONFIG }}
paramFromComponent: {{ .Env.PARAM_FROM_COMPONENT_CONFIG }}
EOF

echo "done!"

echo """
Run the following command to deploy locally built component (or consider adding as a target to makefile):
  DEPLOY_LOCAL_DIR=${local_config_root} LOCALDEV_LOCAL_BUILD=true make local.up local.deploy.${component}
"""
