# See https://docs.earthly.dev/docs/earthfile/features
VERSION --try --raw-output 0.8

PROJECT crossplane/crossplane

ARG --global GO_VERSION=1.23.7

# reviewable checks that a branch is ready for review. Run it before opening a
# pull request. It will catch a lot of the things our CI workflow will catch.
reviewable:
  WAIT
    BUILD +generate
  END
  BUILD +lint
  BUILD +test

# test runs unit tests.
test:
  BUILD +go-test

# lint runs linters.
lint:
  BUILD +go-lint
  BUILD +helm-lint

# build builds Crossplane for your native OS and architecture.
build:
  ARG USERPLATFORM
  BUILD --platform=$USERPLATFORM +go-build
  BUILD +image
  BUILD +helm-build

# multiplatform-build builds Crossplane for all supported OS and architectures.
multiplatform-build:
  BUILD +go-multiplatform-build
  BUILD +multiplatform-image
  BUILD +helm-build

# generate runs code generation. To keep builds fast, it doesn't run as part of
# the build target. It's important to run it explicitly when code needs to be
# generated, for example when you update an API type.
generate:
  BUILD +go-modules-tidy
  BUILD +go-generate
  BUILD +helm-generate

# e2e runs end-to-end tests. See test/e2e/README.md for details.
e2e:
  ARG TARGETARCH
  ARG TARGETOS
  ARG GOARCH=${TARGETARCH}
  ARG GOOS=${TARGETOS}
  ARG FLAGS="-test-suite=base"
  # Using earthly image to allow compatibility with different development environments e.g. WSL
  FROM earthly/dind:alpine-3.20-docker-26.1.5-r0
  RUN wget https://dl.google.com/go/go${GO_VERSION}.${GOOS}-${GOARCH}.tar.gz
  RUN tar -C /usr/local -xzf go${GO_VERSION}.${GOOS}-${GOARCH}.tar.gz
  ENV GOTOOLCHAIN=local
  ENV GOPATH /go
  ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
  RUN apk add --no-cache jq
  COPY +helm-setup/helm /usr/local/bin/helm
  COPY +kind-setup/kind /usr/local/bin/kind
  COPY +gotestsum-setup/gotestsum /usr/local/bin/gotestsum
  # go-build is needed to create the `crank` binary that we use in some of the e2es.
  # technically this will break if we ever used a windows builder (since this would be crank.exe)
  # but we don't so it's fine for now.
  COPY +go-build/crank .
  COPY +go-build-e2e/e2e .
  COPY --dir cluster test .
  TRY
    # Using a static CROSSPLANE_VERSION allows Earthly to cache E2E runs as long
    # as no code changed. If the version contains a git commit (the default) the
    # build layer cache is invalidated on every commit.
    WITH DOCKER --load crossplane-e2e/crossplane:latest=(+image --CROSSPLANE_VERSION=v0.0.0-e2e)
      # TODO(negz:) Set GITHUB_ACTIONS=true and use RUN --raw-output when
      # https://github.com/earthly/earthly/issues/4143 is fixed.
      RUN gotestsum --no-color=false --format testname --junitfile e2e-tests.xml --raw-command go tool test2json -t -p E2E ./e2e -test.v ${FLAGS}
    END
  FINALLY
    SAVE ARTIFACT --if-exists e2e-tests.xml AS LOCAL _output/tests/e2e-tests.xml
  END

# hack builds Crossplane, and deploys it to a kind cluster. It runs in your
# local environment, not a container. The kind cluster will keep running until
# you run the unhack target. Run hack again to rebuild Crossplane and restart
# the kind cluster with the new build.
hack:
  # TODO(negz): This could run an interactive shell inside a temporary container
  # once https://github.com/earthly/earthly/issues/3206 is fixed.
  ARG USERPLATFORM
  ARG SIMULATE_CROSSPLANE_VERSION=v0.0.0-hack
  ARG XPARGS="--debug"
  LOCALLY
  WAIT
    BUILD +unhack
  END
  COPY --platform=${USERPLATFORM} +helm-setup/helm .hack/helm
  COPY --platform=${USERPLATFORM} +kind-setup/kind .hack/kind
  COPY (+helm-build/output --CROSSPLANE_VERSION=v0.0.0-hack) .hack/charts
  WITH DOCKER --load crossplane-hack/crossplane:${SIMULATE_CROSSPLANE_VERSION}=(+image --CROSSPLANE_VERSION=${SIMULATE_CROSSPLANE_VERSION})
    RUN \
      .hack/kind create cluster --name crossplane-hack && \
      .hack/kind load docker-image --name crossplane-hack crossplane-hack/crossplane:${SIMULATE_CROSSPLANE_VERSION} && \
      .hack/helm install --create-namespace --namespace crossplane-system crossplane .hack/charts/crossplane-0.0.0-hack.tgz \
        --set "image.pullPolicy=Never,image.repository=crossplane-hack/crossplane,image.tag=${SIMULATE_CROSSPLANE_VERSION}" \
        --set "args={${XPARGS}}"
  END
  RUN docker image rm crossplane-hack/crossplane:${SIMULATE_CROSSPLANE_VERSION}
  RUN rm -rf .hack

# unhack deletes the kind cluster created by the hack target.
unhack:
  ARG USERPLATFORM
  LOCALLY
  COPY --platform=${USERPLATFORM} +kind-setup/kind .hack/kind
  RUN .hack/kind delete cluster --name crossplane-hack
  RUN rm -rf .hack

# go-modules downloads Crossplane's go modules. It's the base target of most Go
# related target (go-build, etc).
go-modules:
  ARG NATIVEPLATFORM
  FROM --platform=${NATIVEPLATFORM} golang:${GO_VERSION}
  WORKDIR /crossplane
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY go.mod go.sum ./
  RUN go mod download
  SAVE ARTIFACT go.mod AS LOCAL go.mod
  SAVE ARTIFACT go.sum AS LOCAL go.sum

# go-modules-tidy tidies and verifies go.mod and go.sum.
go-modules-tidy:
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ cmd/ internal/ pkg/ test/ .
  RUN go mod tidy
  RUN go mod verify
  SAVE ARTIFACT go.mod AS LOCAL go.mod
  SAVE ARTIFACT go.sum AS LOCAL go.sum

# go-generate runs Go code generation.
go-generate:
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY +kubectl-setup/kubectl /usr/local/bin/kubectl
  COPY --dir cluster/crd-patches cluster/crd-patches
  COPY --dir hack/ apis/ internal/ .
  RUN go generate -tags 'generate' ./apis/...
  # TODO(negz): Can this move into generate.go? Ideally it would live there with
  # the code that actually generates the CRDs, but it depends on kubectl.
  RUN kubectl patch --local --type=json \
    --patch-file cluster/crd-patches/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
    --filename cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
    --output=yaml > /tmp/patched.yaml \
    && mv /tmp/patched.yaml cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml
  SAVE ARTIFACT apis/ AS LOCAL apis
  SAVE ARTIFACT cluster/crds AS LOCAL cluster/crds
  SAVE ARTIFACT cluster/meta AS LOCAL cluster/meta

# go-build builds Crossplane binaries for your native OS and architecture.
go-build:
  ARG EARTHLY_GIT_SHORT_HASH
  ARG EARTHLY_GIT_COMMIT_TIMESTAMP
  ARG CROSSPLANE_VERSION=v0.0.0-${EARTHLY_GIT_COMMIT_TIMESTAMP}-${EARTHLY_GIT_SHORT_HASH}
  ARG TARGETARCH
  ARG TARGETOS
  ARG GOARCH=${TARGETARCH}
  ARG GOOS=${TARGETOS}
  ARG GOFLAGS="\"-ldflags=-s -w -X=github.com/crossplane/crossplane/internal/version.version=${CROSSPLANE_VERSION}\""
  ARG CGO_ENABLED=0
  FROM +go-modules
  LET ext = ""
  IF [ "$GOOS" = "windows" ]
    SET ext = ".exe"
  END
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ cmd/ internal/ pkg/ .
  RUN go build -o crossplane${ext} ./cmd/crossplane
  RUN sha256sum crossplane${ext} | head -c 64 > crossplane${ext}.sha256
  RUN go build -o crank${ext} ./cmd/crank
  RUN sha256sum crank${ext} | head -c 64 > crank${ext}.sha256
  RUN tar -czvf crank.tar.gz crank${ext} crank${ext}.sha256
  RUN sha256sum crank.tar.gz | head -c 64 > crank.tar.gz.sha256
  SAVE ARTIFACT --keep-ts crossplane${ext} AS LOCAL _output/bin/${GOOS}_${GOARCH}/crossplane${ext}
  SAVE ARTIFACT --keep-ts crossplane${ext}.sha256 AS LOCAL _output/bin/${GOOS}_${GOARCH}/crossplane${ext}.sha256
  SAVE ARTIFACT --keep-ts crank${ext} AS LOCAL _output/bin/${GOOS}_${GOARCH}/crank${ext}
  SAVE ARTIFACT --keep-ts crank${ext}.sha256 AS LOCAL _output/bin/${GOOS}_${GOARCH}/crank${ext}.sha256
  SAVE ARTIFACT --keep-ts crank.tar.gz AS LOCAL _output/bundle/${GOOS}_${GOARCH}/crank.tar.gz
  SAVE ARTIFACT --keep-ts crank.tar.gz.sha256 AS LOCAL _output/bundle/${GOOS}_${GOARCH}/crank.tar.gz.sha256

# go-multiplatform-build builds Crossplane binaries for all supported OS
# and architectures.
go-multiplatform-build:
  BUILD \
    --platform=linux/amd64 \
    --platform=linux/arm64 \
    --platform=linux/arm \
    --platform=linux/ppc64le \
    --platform=darwin/arm64 \
    --platform=darwin/amd64 \
    --platform=windows/amd64 \
    +go-build

# go-build-e2e builds Crossplane's end-to-end tests.
go-build-e2e:
  ARG CGO_ENABLED=0
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ internal/ test/ .
  RUN go test -c -o e2e ./test/e2e
  SAVE ARTIFACT e2e

# go-test runs Go unit tests.
go-test:
  ARG KUBE_VERSION=1.30.3
  FROM +go-modules
  DO github.com/earthly/lib+INSTALL_DIND
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ cmd/ internal/ pkg/ cluster/ .
  COPY --dir +envtest-setup/envtest /usr/local/kubebuilder/bin
  # a bit dirty but preload the cache with the images we use in IT (found in functions.yaml)
  WITH DOCKER \
    --pull xpkg.upbound.io/crossplane-contrib/function-go-templating:v0.9.0 \
    --pull xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.4.2 \
    --pull xpkg.upbound.io/crossplane-contrib/function-environment-configs:v0.2.0 \
    --pull xpkg.upbound.io/crossplane-contrib/function-extra-resources:v0.0.3
    # this is silly, but we put these files into the default KUBEBUILDER_ASSETS location, because if we set
    # KUBEBUILDER_ASSETS on `go test` to the artifact path, which is perhaps more intuitive, the syntax highlighting
    # in intellij breaks due to the word BUILD in all caps.
    RUN go test -covermode=count -coverprofile=coverage.txt ./...
  END
  SAVE ARTIFACT coverage.txt AS LOCAL _output/tests/coverage.txt

# go-lint lints Go code.
go-lint:
  ARG GOLANGCI_LINT_VERSION=v1.62.2
  FROM +go-modules
  # This cache is private because golangci-lint doesn't support concurrent runs.
  CACHE --id go-lint --sharing private /root/.cache/golangci-lint
  CACHE --id go-build --sharing shared /root/.cache/go-build
  RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCI_LINT_VERSION}
  COPY .golangci.yml .
  COPY --dir apis/ cmd/ internal/ pkg/ test/ .
  RUN golangci-lint run --fix
  SAVE ARTIFACT apis AS LOCAL apis
  SAVE ARTIFACT cmd AS LOCAL cmd
  SAVE ARTIFACT internal AS LOCAL internal
  SAVE ARTIFACT pkg AS LOCAL pkg
  SAVE ARTIFACT test AS LOCAL test

# image builds the Crossplane OCI image for your native architecture.
image:
  ARG EARTHLY_GIT_BRANCH
  ARG EARTHLY_GIT_SHORT_HASH
  ARG EARTHLY_GIT_COMMIT_TIMESTAMP
  ARG CROSSPLANE_REPO=build-${EARTHLY_GIT_SHORT_HASH}/crossplane
  ARG CROSSPLANE_VERSION=v0.0.0-${EARTHLY_GIT_COMMIT_TIMESTAMP}-${EARTHLY_GIT_SHORT_HASH}
  ARG NATIVEPLATFORM
  ARG TARGETPLATFORM
  ARG TARGETARCH
  ARG TARGETOS
  FROM --platform=${TARGETPLATFORM} gcr.io/distroless/static@sha256:5c7e2b465ac6a2a4e5f4f7f722ce43b147dabe87cb21ac6c4007ae5178a1fa58
  COPY --platform=${NATIVEPLATFORM} (+go-build/crossplane --GOOS=${TARGETOS} --GOARCH=${TARGETARCH}) /usr/local/bin/
  COPY --dir cluster/crds/ /crds
  COPY --dir cluster/webhookconfigurations/ /webhookconfigurations
  EXPOSE 8080
  USER 65532
  ENTRYPOINT ["crossplane"]
  SAVE IMAGE --push ${CROSSPLANE_REPO}:${CROSSPLANE_VERSION}
  SAVE IMAGE --push ${CROSSPLANE_REPO}:${EARTHLY_GIT_BRANCH}

# multiplatform-image builds the Crossplane OCI image for all supported
# architectures.
multiplatform-image:
  BUILD \
    --platform=linux/amd64 \
    --platform=linux/arm64 \
    --platform=linux/arm \
    --platform=linux/ppc64le \
    +image

# helm-lint lints the Crossplane Helm chart.
helm-lint:
  FROM alpine:3.20
  WORKDIR /chart
  COPY +helm-setup/helm /usr/local/bin/helm
  COPY cluster/charts/crossplane/ .
  RUN --entrypoint helm lint

# helm-generate runs Helm code generation - specifically helm-docs.
helm-generate:
  FROM alpine:3.20
  WORKDIR /chart
  COPY +helm-docs-setup/helm-docs /usr/local/bin/helm-docs
  COPY cluster/charts/crossplane/ .
  RUN helm-docs
  SAVE ARTIFACT . AS LOCAL cluster/charts/crossplane

# helm-build packages the Crossplane Helm chart.
helm-build:
  ARG EARTHLY_GIT_SHORT_HASH
  ARG EARTHLY_GIT_COMMIT_TIMESTAMP
  ARG CROSSPLANE_VERSION=v0.0.0-${EARTHLY_GIT_COMMIT_TIMESTAMP}-${EARTHLY_GIT_SHORT_HASH}
  FROM alpine:3.20
  WORKDIR /chart
  COPY +helm-setup/helm /usr/local/bin/helm
  COPY cluster/charts/crossplane/ .
  # We strip the leading v from Helm chart versions.
  LET CROSSPLANE_CHART_VERSION=$(echo ${CROSSPLANE_VERSION}|sed -e 's/^v//')
  RUN helm dependency update
  RUN helm package --version ${CROSSPLANE_CHART_VERSION} --app-version ${CROSSPLANE_CHART_VERSION} -d output .
  SAVE ARTIFACT output AS LOCAL _output/charts

# envtest-setup is used by other targets to setup envtest.
envtest-setup:
  ARG KUBE_VERSION=1.30.3
  ARG TARGETOS
  ARG TARGETARCH
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  # pin for golang 1.23.  when upgrading to 1.24, upgrade this version to latest(ish):
  RUN go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.20
  RUN setup-envtest use ${KUBE_VERSION} --os ${TARGETOS} --arch ${TARGETARCH} --bin-dir ./envtest
  SAVE ARTIFACT ./envtest/k8s/${KUBE_VERSION}-${TARGETOS}-${TARGETARCH} ./envtest

# kubectl-setup is used by other targets to setup kubectl.
kubectl-setup:
  ARG KUBECTL_VERSION=v1.30.1
  ARG NATIVEPLATFORM
  ARG TARGETOS
  ARG TARGETARCH
  FROM --platform=${NATIVEPLATFORM} curlimages/curl:8.8.0
  RUN curl -fsSL https://dl.k8s.io/${KUBECTL_VERSION}/kubernetes-client-${TARGETOS}-${TARGETARCH}.tar.gz|tar zx
  SAVE ARTIFACT kubernetes/client/bin/kubectl

# kind-setup is used by other targets to setup kind.
kind-setup:
  ARG KIND_VERSION=v0.25.0
  ARG NATIVEPLATFORM
  ARG TARGETOS
  ARG TARGETARCH
  FROM --platform=${NATIVEPLATFORM} curlimages/curl:8.8.0
  RUN curl -fsSLo kind https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-${TARGETOS}-${TARGETARCH}&&chmod +x kind
  SAVE ARTIFACT kind

# gotestsum-setup is used by other targets to setup gotestsum.
gotestsum-setup:
  ARG GOTESTSUM_VERSION=1.12.0
  ARG NATIVEPLATFORM
  ARG TARGETOS
  ARG TARGETARCH
  FROM --platform=${NATIVEPLATFORM} curlimages/curl:8.8.0
  RUN curl -fsSL https://github.com/gotestyourself/gotestsum/releases/download/v${GOTESTSUM_VERSION}/gotestsum_${GOTESTSUM_VERSION}_${TARGETOS}_${TARGETARCH}.tar.gz|tar zx>gotestsum
  SAVE ARTIFACT gotestsum

# helm-docs-setup is used by other targets to setup helm-docs.
helm-docs-setup:
  ARG HELM_DOCS_VERSION=1.14.2
  ARG NATIVEPLATFORM
  ARG TARGETOS
  ARG TARGETARCH
  FROM --platform=${NATIVEPLATFORM} curlimages/curl:8.8.0
  IF [ "${TARGETARCH}" = "amd64" ]
    LET ARCH=x86_64
  ELSE
    LET ARCH=${TARGETARCH}
  END
  RUN curl -fsSL https://github.com/norwoodj/helm-docs/releases/download/v${HELM_DOCS_VERSION}/helm-docs_${HELM_DOCS_VERSION}_${TARGETOS}_${ARCH}.tar.gz|tar zx>helm-docs
  SAVE ARTIFACT helm-docs

# helm-setup is used by other targets to setup helm.
helm-setup:
  ARG HELM_VERSION=v3.16.3
  ARG NATIVEPLATFORM
  ARG TARGETOS
  ARG TARGETARCH
  FROM --platform=${NATIVEPLATFORM} curlimages/curl:8.8.0
  RUN curl -fsSL https://get.helm.sh/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz|tar zx --strip-components=1
  SAVE ARTIFACT helm

# Targets below this point are intended only for use in GitHub Actions CI. They
# may not work outside of that environment. For example they may depend on
# secrets that are only availble in the CI environment. Targets below this point
# must be prefixed with ci-.

# TODO(negz): Is there a better way to determine the Crossplane version?
# This versioning approach maintains compatibility with the build submodule. See
# https://github.com/crossplane/build/blob/231258/makelib/common.mk#L205. This
# approach is problematic in Earthly because computing it inside a containerized
# target requires copying the entire git repository into the container. Doing so
# would invalidate all dependent target caches any time any file in git changed.

# ci-version is used by CI to set the CROSSPLANE_VERSION environment variable.
ci-version:
  LOCALLY
  RUN echo "CROSSPLANE_VERSION=$(git describe --dirty --always --tags|sed -e 's/-/./2g')" > $GITHUB_ENV

# ci-artifacts is used by CI to build and push the Crossplane image, chart, and
# binaries.
ci-artifacts:
  BUILD +multiplatform-build \
    --CROSSPLANE_REPO=index.docker.io/crossplane/crossplane \
    --CROSSPLANE_REPO=ghcr.io/crossplane/crossplane \
    --CROSSPLANE_REPO=xpkg.upbound.io/crossplane/crossplane

# ci-codeql-setup sets up CodeQL for the ci-codeql target.
ci-codeql-setup:
  ARG CODEQL_VERSION=v2.19.3
  FROM curlimages/curl:8.8.0
  RUN curl -fsSL https://github.com/github/codeql-action/releases/download/codeql-bundle-${CODEQL_VERSION}/codeql-bundle-linux64.tar.gz|tar zx
  SAVE ARTIFACT codeql

# ci-codeql is used by CI to build Crossplane with CodeQL scanning enabled.
ci-codeql:
  ARG CGO_ENABLED=0
  ARG TARGETOS
  ARG TARGETARCH
  # Using a static CROSSPLANE_VERSION allows Earthly to cache E2E runs as long
  # as no code changed. If the version contains a git commit (the default) the
  # build layer cache is invalidated on every commit.
  FROM +go-modules --CROSSPLANE_VERSION=v0.0.0-codeql
  IF [ "${TARGETARCH}" = "arm64" ] && [ "${TARGETOS}" = "linux" ]
    RUN --no-cache echo "CodeQL doesn't support Linux on Apple Silicon" && false
  END
  COPY --dir +ci-codeql-setup/codeql /codeql
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ cmd/ internal/ pkg/ .
  RUN /codeql/codeql database create /codeqldb --language=go
  RUN /codeql/codeql database analyze /codeqldb --threads=0 --format=sarif-latest --output=go.sarif --sarif-add-baseline-file-info
  SAVE ARTIFACT go.sarif AS LOCAL _output/codeql/go.sarif

# ci-promote-image is used by CI to promote a Crossplane image to a channel.
# In practice, this means creating a new channel tag (e.g. master or stable)
# that points to the supplied version.
ci-promote-image:
  ARG --required CROSSPLANE_REPO
  ARG --required CROSSPLANE_VERSION
  ARG --required CHANNEL
  FROM alpine:3.20
  RUN apk add docker
  # We need to omit the registry argument when we're logging into Docker Hub.
  # Otherwise login will appear to succeed, but buildx will fail on auth.
  IF [[ "${CROSSPLANE_REPO}" == *docker.io/* ]]
    RUN --secret DOCKER_USER --secret DOCKER_PASSWORD docker login -u ${DOCKER_USER} -p ${DOCKER_PASSWORD}
  ELSE
    RUN --secret DOCKER_USER --secret DOCKER_PASSWORD docker login -u ${DOCKER_USER} -p ${DOCKER_PASSWORD} ${CROSSPLANE_REPO}
  END
  RUN --push docker buildx imagetools create \
    --tag ${CROSSPLANE_REPO}:${CHANNEL} \
    --tag ${CROSSPLANE_REPO}:${CROSSPLANE_VERSION}-${CHANNEL} \
    ${CROSSPLANE_REPO}:${CROSSPLANE_VERSION}

# TODO(negz): Ideally ci-push-build-artifacts would be merged into ci-artifacts,
# i.e. just build and push them all in the same target. Currently we're relying
# on the fact that ci-artifacts does a bunch of SAVE ARTIFACT AS LOCAL, which
# ci-push-build-artifacts then loads. That's an anti-pattern in Earthly. We're
# supposed to use COPY instead, but I'm not sure how to COPY artifacts from a
# matrix build.

# ci-push-build-artifacts is used by CI to push binary artifacts to S3.
ci-push-build-artifacts:
  ARG --required CROSSPLANE_VERSION
  ARG --required BUILD_DIR
  ARG ARTIFACTS_DIR=_output
  ARG BUCKET_RELEASES=crossplane.releases
  ARG AWS_DEFAULT_REGION
  FROM amazon/aws-cli:2.15.61
  COPY --dir ${ARTIFACTS_DIR} artifacts
  RUN --push --secret=AWS_ACCESS_KEY_ID --secret=AWS_SECRET_ACCESS_KEY aws s3 sync --delete --only-show-errors artifacts s3://${BUCKET_RELEASES}/build/${BUILD_DIR}/${CROSSPLANE_VERSION}

# ci-promote-build-artifacts is used by CI to promote binary artifacts and Helm
# charts to a channel. In practice, this means copying them from one S3
# directory to another.
ci-promote-build-artifacts:
  ARG --required CROSSPLANE_VERSION
  ARG --required BUILD_DIR
  ARG --required CHANNEL
  ARG HELM_REPO_URL=https://charts.crossplane.io
  ARG BUCKET_RELEASES=crossplane.releases
  ARG BUCKET_CHARTS=crossplane.charts
  ARG PRERELEASE=false
  ARG AWS_DEFAULT_REGION
  FROM amazon/aws-cli:2.15.61
  COPY +helm-setup/helm /usr/local/bin/helm
  RUN --secret=AWS_ACCESS_KEY_ID --secret=AWS_SECRET_ACCESS_KEY aws s3 sync --only-show-errors s3://${BUCKET_CHARTS}/${CHANNEL} repo
  RUN --secret=AWS_ACCESS_KEY_ID --secret=AWS_SECRET_ACCESS_KEY aws s3 sync --only-show-errors s3://${BUCKET_RELEASES}/build/${BUILD_DIR}/${CROSSPLANE_VERSION}/charts repo
  RUN helm repo index --url ${HELM_REPO_URL}/${CHANNEL} repo
  RUN --push --secret=AWS_ACCESS_KEY_ID --secret=AWS_SECRET_ACCESS_KEY aws s3 sync --delete --only-show-errors repo s3://${BUCKET_CHARTS}/${CHANNEL}
  RUN --push --secret=AWS_ACCESS_KEY_ID --secret=AWS_SECRET_ACCESS_KEY aws s3 cp --only-show-errors --cache-control "private, max-age=0, no-transform" repo/index.yaml s3://${BUCKET_CHARTS}/${CHANNEL}/index.yaml
  RUN --push --secret=AWS_ACCESS_KEY_ID --secret=AWS_SECRET_ACCESS_KEY aws s3 sync --delete --only-show-errors s3://${BUCKET_RELEASES}/build/${BUILD_DIR}/${CROSSPLANE_VERSION} s3://${BUCKET_RELEASES}/${CHANNEL}/${CROSSPLANE_VERSION}
  IF [ "${PRERELEASE}" = "false" ]
    RUN --push --secret=AWS_ACCESS_KEY_ID --secret=AWS_SECRET_ACCESS_KEY aws s3 sync --delete --only-show-errors s3://${BUCKET_RELEASES}/build/${BUILD_DIR}/${CROSSPLANE_VERSION} s3://${BUCKET_RELEASES}/${CHANNEL}/current
  END
