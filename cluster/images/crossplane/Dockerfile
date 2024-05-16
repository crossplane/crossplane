FROM gcr.io/distroless/static@sha256:41972110a1c1a5c0b6adb283e8aa092c43c31f7c5d79b8656fbffff2c3e61f05

ARG TARGETOS
ARG TARGETARCH

COPY bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
COPY crds /crds
COPY webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]
