FROM gcr.io/distroless/static@sha256:7e5c6a2a4ae854242874d36171b31d26e0539c98fc6080f942f16b03e82851ab

ARG TARGETOS
ARG TARGETARCH

COPY bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
COPY crds /crds
COPY webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]
