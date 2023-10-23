FROM gcr.io/distroless/static@sha256:6706c73aae2afaa8201d63cc3dda48753c09bcd6c300762251065c0f7e602b25

ARG TARGETOS
ARG TARGETARCH

COPY bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
COPY crds /crds
COPY webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]
