# Built by test/e2e/functions/ordering/build.sh, which cross-compiles the
# binary first so this stage stays a plain copy.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETARCH
COPY function-linux-${TARGETARCH} /usr/local/bin/function
USER 65532
ENTRYPOINT ["function"]
