#!/bin/sh

# Usage example:
#
#  ./duplicate_proto_type.sh apiextensions/fn/proto/v1/run_function.proto apiextensions/fn/proto/v1beta1
#
# The above command will create zz_generated.run_function.proto in the v1beta1
# directory. The most specific segment of the package name is assumed to be the
# same as the target directory (i.e. v1beta1).

set -e

FROM_PATH=${1}
TO_DIR=${2}

DO_NOT_EDIT="// Generated from ${FROM_PATH} by ${0}. DO NOT EDIT."

FROM_DIR=$(dirname ${FROM_PATH})
FROM_FILE=$(basename ${FROM_PATH})
FROM_PACKAGE=$(basename ${FROM_DIR})

TO_PACKAGE=$(basename ${TO_DIR})
TO_PATH="${TO_DIR}/zz_generated_${FROM_FILE}"

sed -r \
  -e "s#^package (.+)\.${FROM_PACKAGE};\$#${DO_NOT_EDIT}\n\npackage \1.${TO_PACKAGE};#" \
  -e "s#^option go_package = \"(.+)/${FROM_PACKAGE}\";\$#option go_package = \"\1/${TO_PACKAGE}\";#" \
  ${FROM_PATH} > ${TO_PATH}

echo "Duplicated ${FROM_PATH} (package ${FROM_PACKAGE}) to ${TO_PATH} (package ${TO_PACKAGE})."
