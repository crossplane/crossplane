#!/bin/bash -eu
#
# IMPORTANT: Fuzz* test cases should be in a dedicated file, conventionally
# called `fuzz_test.go`, but that's not a requirement. Otherwise once the file
# name is changed to have the _test_fuzz.go termination instead of _test.go as
# required by oss-fuzz, the code won't compile as other Test* test cases might
# not find some requirements given that they are not in a _test.go file.
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
  compile_native_go_fuzzer github.com/crossplane/crossplane-runtime/$folder $func $func
'
