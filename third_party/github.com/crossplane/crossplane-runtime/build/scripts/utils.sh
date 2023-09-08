################################# setup colors
# setting up colors
BOLD='\033[10;1m'
YLW='\033[0;33m'
GRN='\033[0;32m'
RED='\033[0;31m'
NOC='\033[0m' # No Color

echo_info(){
    msg="${1}"
    [[ -n ${COMPONENT:-} ]] && msg="[${COMPONENT:-}] ${msg}"
    printf "${BOLD}%s${NOC}\n" "${msg}"
}

echo_success(){
    msg="${1}"
    [[ -n ${COMPONENT:-} ]] && msg="[${COMPONENT:-}] ${msg}"
    printf "${GRN}%s${NOC}\n" "${msg}"
}

echo_warn(){
    msg="${1}"
    [[ -n ${COMPONENT:-} ]] && msg="[${COMPONENT:-}] ${msg}"
    printf "${YLW}%s${NOC}\n" "${msg}"
}

echo_error(){
    msg="${1}"
    [[ -n ${COMPONENT:-} ]] && msg="[${COMPONENT:-}] ${msg}"
    printf "\n${RED}%s${NOC}\n" "${msg}"
    return 1
}

#################################

containsElement () {
  local e match="$1"
  shift
  for e; do [[ "$e" == "$match" ]] && return 0; done
  return 1
}

pullAndLoadImage () {
  i=$1
  # Pull the image:
  # - if has a tag "master" or "latest"
  # - or does not exist already.
  if echo "${i}" | grep ":master\s*$" >/dev/null || echo "${i}" | grep ":latest\s*$" >/dev/null || \
    ! docker inspect --type=image "${i}" >/dev/null 2>&1; then
    docker pull "${i}"
  fi
  "${KIND}" load docker-image "${i}" --name="${KIND_CLUSTER_NAME}"
  return 0
}

createNamespace () {
  n=$1
  # Create namespace if not exists
  "${KUBECTL}" --kubeconfig "${KUBECONFIG}" get ns "${n}" >/dev/null 2>&1 || \
    ${KUBECTL} --kubeconfig "${KUBECONFIG}" create ns "${n}"
}