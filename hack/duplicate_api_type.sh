#!/bin/sh

# Usage example:
#
#  ./duplicate_api_type.sh apiextensions/v1beta1/revision_types.go apiextensions/v1alpha1
#
# The above command will create zz_generated.revision_types.go in the v1alpha1
# directory. The package name is assumed to be the same as the target directory
# (i.e. v1alpha1). The duplicate API type cannot be a storage version - the
# +kubebuilder:storageversion comment marker will be discarded.

set -e

FROM_PATH=${1}
TO_DIR=${2}
STORAGE_VERSION=${3:-false}

DO_NOT_EDIT="// Generated from ${FROM_PATH} by ${0}. DO NOT EDIT."

FROM_DIR=$(dirname ${FROM_PATH})
FROM_FILE=$(basename ${FROM_PATH})
FROM_PACKAGE=$(basename ${FROM_DIR})

TO_PACKAGE=$(basename ${TO_DIR})
TO_PATH="${TO_DIR}/zz_generated.${FROM_FILE}"

sed "s#^package ${FROM_PACKAGE}\$#${DO_NOT_EDIT}\n\npackage ${TO_PACKAGE}#" ${FROM_PATH} > ${TO_PATH}

case $STORAGE_VERSION in
  true)
    if grep -q "+kubebuilder:storageversion" ${FROM_PATH}; then
      echo "Error: ${FROM_PATH} is marked as storage version and cannot be duplicated without dropping the marker."
      exit 1
    fi
    # Add the storageVersion marker before // +genclient
    sed -i '\/\/ +genclient$/i\/\/ +kubebuilder:storageversion' ${TO_PATH}
    echo "Duplicated ${FROM_PATH} (package ${FROM_PACKAGE}) to ${TO_PATH} (package ${TO_PACKAGE})."
    ;;
  false)
    # Remove the +kubebuilder:storageversion comment marker
    sed -i '/+kubebuilder:storageversion/d' ${TO_PATH}
    echo "Duplicated ${FROM_PATH} (package ${FROM_PACKAGE}) to ${TO_PATH} (package ${TO_PACKAGE}), removed storage version marker."
    ;;
  *)
    echo "Error: Invalid STORAGE_VERSION value: ${STORAGE_VERSION}"
    exit 1
    ;;
esac