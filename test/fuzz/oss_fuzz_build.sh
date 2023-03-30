#!/bin/bash -eu
#
# DO NOT DELETE: this script is used from oss-fuzz. You can find more details
# in the official documentation:
# https://google.github.io/oss-fuzz/getting-started/new-project-guide/go-lang/
#
# To run this locally you can go through the following steps: - $ git clone
# https://github.com/google/oss-fuzz --depth=1 - $ cd
# oss-fuzz/projects/crossplane
# - modify Dockerfile to point to your branch with all the fuzzers being merged.
# - modify build.sh to call the build script in Crossplanes repository
# - $ python3 ../../infra/helper.py build_image crossplane
# - $ python3 ../../infra/helper.py build_fuzzers crossplane
#
# ASSUMPTIONS:
# This script's only assumption is that all 'Fuzz*' test cases MUST live in a
# separate '*_test.go' file with respect to other 'Test*' functions and that
# all functions it requires MUST either be defined in the same file or in non
# '*_test.go' files. This is because, due to how OSS-Fuzz builds fuzzers, we
# have to rename files defining 'Fuzz*' functions to be in non '*_test.go'
# files and therefore, once moved, the tests won't have access to functions
# defined in other '*_test.go' files as usual for Go code. The files are
# usually named 'fuzz_test.go', but that's not mandatory as the script is
# automatically finding any file containing at least one 'Fuzz*' test case.
# Multiple 'Fuzz*' test cases can live in the same file, the script will build
# separate fuzzers for each of them as required.

set -o nounset
set -o pipefail
set -o errexit
set -x

printf "package main\nimport ( \n _ \"github.com/AdamKorcz/go-118-fuzz-build/testing\"\n )\n" > register.go

# Moving all the fuzz_test.go to fuzz_test_fuzz.go, as oss-fuzz uses go build to build fuzzers
# shellcheck disable=SC2016
grep --line-buffered --include '*_test.go' -Pr 'func Fuzz.*\(.* \*testing\.F' | cut -d: -f1 | sort -u | xargs -I{} sh -c '
  file="{}"
  file_no_ext="$(basename "$file" | cut -d"." -f1)"
  folder="$(dirname $file)"
  mv "$file" "$folder/${file_no_ext}_fuzz.go"
'

# Now we can tidy and download all our dependencies
go mod tidy
go mod vendor

# Find all native fuzzers and compile them
# shellcheck disable=SC2016
grep --line-buffered --include '*_test_fuzz.go' -Pr 'func Fuzz.*\(.* \*testing\.F' | sed -E 's/(func Fuzz(.*)\(.*)/\2/' | xargs -I{} sh -c '
  file="$(echo "{}" | cut -d: -f1)"
  folder="$(dirname $file)"
  func="Fuzz$(echo "{}" | cut -d: -f2)"
  compile_native_go_fuzzer github.com/crossplane/crossplane/$folder $func $func
'
