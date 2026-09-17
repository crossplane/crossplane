#!/usr/bin/env bash
# Build and optionally push the ordering test function as a Crossplane package.
#
# The function is compiled against this repo's proto/fn/v1, so it must be
# rebuilt whenever the Dependency messages change. Tag accordingly - a stale
# published image fails end-to-end tests in confusing ways.
#
# Usage:
#   ./build.sh                          # build only
#   ./build.sh index.docker.io/you/function-ordering:tag   # build and push
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "${here}/../../../.." && pwd)"
out="${here}/_output"
image="${1:-}"

rm -rf "${out}"
mkdir -p "${out}"

for arch in amd64 arm64; do
	echo "==> compiling linux/${arch}"
	(cd "${repo}" && CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" \
		go build -o "${out}/function-linux-${arch}" ./test/e2e/functions/ordering)

	echo "==> building runtime image for linux/${arch}"
	cp "${here}/Dockerfile" "${out}/Dockerfile"
	docker build --quiet --platform "linux/${arch}" \
		--build-arg "TARGETARCH=${arch}" \
		-t "function-ordering-runtime:${arch}" \
		-f "${out}/Dockerfile" "${out}" >/dev/null

	echo "==> building xpkg for linux/${arch}"
	crossplane xpkg build \
		--package-root="${here}/package" \
		--embed-runtime-image="function-ordering-runtime:${arch}" \
		--package-file="${out}/function-ordering-${arch}.xpkg"
done

if [ -z "${image}" ]; then
	echo "built: ${out}/function-ordering-{amd64,arm64}.xpkg (not pushed)"
	exit 0
fi

echo "==> pushing ${image}"
crossplane xpkg push \
	--package-files="${out}/function-ordering-amd64.xpkg,${out}/function-ordering-arm64.xpkg" \
	"${image}"
