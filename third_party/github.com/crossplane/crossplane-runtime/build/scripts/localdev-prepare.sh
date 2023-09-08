#!/usr/bin/env bash
set -aeuo pipefail

# Source utility functions
source "${SCRIPTS_DIR}/utils.sh"

getRepo() {
  repo_config=$1

  IFS='@' read -ra repo_and_branch <<< "${repo_config}"
  repo="${repo_and_branch[0]}"
  branch="${repo_and_branch[1]:-}"
  [[ -z "${branch}" ]] && branch="master"

  repo_dir=$(basename "${repo}" .git)
  repo_cache_dir="${LOCALDEV_WORKDIR_REPOS}/${repo_dir}"

  if ! [ -d "${repo_cache_dir}" ]; then
    echo_info "Cloning branch \"${branch}\" of repo \"${repo}\"..."
    repo_url="git@github.com:${repo}.git"
    if [ "${LOCALDEV_CLONE_WITH}" == "https" ]; then
      repo_url="https://github.com/${repo}.git"
    fi
    git clone --depth 1 "${repo_url}" "${repo_cache_dir}" -b "${branch}"
    echo_info "Cloning branch \"${branch}\" of repo \"${repo}\"...OK"
  elif [ "${LOCALDEV_PULL_LATEST}" == "true" ]; then
    echo_info "Getting latest branch \"${branch}\" of repo \"${repo}\"..."
    git -C "${repo_cache_dir}" stash > /dev/null
    git -C "${repo_cache_dir}" fetch origin
    if ! output=$(git -C "${repo_cache_dir}" reset --hard origin/"${branch}" 2>&1); then
      echo_error "${output}"
    fi

    echo_info "Getting latest branch \"${branch}\" of repo \"${repo}\"...OK"
  fi
}

###

# prepare local dev configuration under ".work/local" by gathering configuration from different repositories.
LOCALDEV_WORKDIR=${WORK_DIR}/local
mkdir -p "${LOCALDEV_WORKDIR}"
LOCALDEV_WORKDIR_REPOS=${LOCALDEV_WORKDIR}/repos

if [ -z "${LOCALDEV_INTEGRATION_CONFIG_REPO}" ]; then
  echo_info "No integration config repo configured, using local config"
  mkdir -p "${DEPLOY_LOCAL_WORKDIR}"
  cp -rf "${DEPLOY_LOCAL_DIR}/." "${DEPLOY_LOCAL_WORKDIR}"
else
  echo_info "Using integration config from repo@branch \"${LOCALDEV_INTEGRATION_CONFIG_REPO}\""
  getRepo "${LOCALDEV_INTEGRATION_CONFIG_REPO}"
  mkdir -p "${DEPLOY_LOCAL_WORKDIR}"
  cp -rf "${repo_cache_dir}/." "${DEPLOY_LOCAL_WORKDIR}"
fi

if [ -n "${LOCAL_DEV_REPOS}" ]; then
  repositories_arr=($LOCAL_DEV_REPOS)
  for i in ${repositories_arr[@]+"${repositories_arr[@]}"}; do

    local_repo=$(basename $(git config --get remote.origin.url) .git)
    base_repo=$(basename "${i}" .git)

    if [ "${LOCALDEV_LOCAL_BUILD}" == "true" ] && [ "${base_repo}" == "${local_repo}" ]; then
      # if it is a local build and repo is the local one, just use local config

      echo_info "Using local config for repo \"${base_repo}\""
      repo_dir="${ROOT_DIR}"
    else
      getRepo "${i}"
      repo_dir=${LOCALDEV_WORKDIR_REPOS}/${base_repo}
    fi

    # copy local dev config under workdir
    local_config_dir="${repo_dir}/cluster/local/config"
    if [ -d "${local_config_dir}" ]; then
      cp -rf "${local_config_dir}/." "${DEPLOY_LOCAL_WORKDIR}/config"
    else
      echo_warn "No local dev config found for repo \"${base_repo}\""
    fi
  done
fi

# prepare post-render workdir
mkdir -p "${DEPLOY_LOCAL_POSTRENDER_WORKDIR}"

localdev_postrender_kustomization="${DEPLOY_LOCAL_POSTRENDER_WORKDIR}/kustomization.yaml"
cat << EOF > "${localdev_postrender_kustomization}"
resources:
  - in.yaml
patches:
  - path: patch-deployment.yaml
    target:
      kind: Deployment
      name: ".*"
  - path: patch-rollout.yaml
    target:
      kind: Rollout
      name: ".*"
EOF

localdev_postrender_patch_deployment="${DEPLOY_LOCAL_POSTRENDER_WORKDIR}/patch-deployment.yaml"
cat << EOF > "${localdev_postrender_patch_deployment}"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: any
spec:
  template:
    metadata:
      annotations:
        rollme: "$RANDOM"
EOF

localdev_postrender_patch_rollout="${DEPLOY_LOCAL_POSTRENDER_WORKDIR}/patch-rollout.yaml"
cat << EOF > "${localdev_postrender_patch_rollout}"
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: any
spec:
  template:
    metadata:
      annotations:
        rollme: "$RANDOM"
EOF

LOCALDEV_POSTRENDER_EXEC="${DEPLOY_LOCAL_POSTRENDER_WORKDIR}/exec"
cat << EOF > "${LOCALDEV_POSTRENDER_EXEC}"
#!/bin/bash

cat <&0 > ${DEPLOY_LOCAL_POSTRENDER_WORKDIR}/in.yaml

${KUSTOMIZE} build ${DEPLOY_LOCAL_POSTRENDER_WORKDIR}
EOF
chmod +x "${LOCALDEV_POSTRENDER_EXEC}"
