FROM gcr.io/distroless/static@sha256:e7e79fb2947f38ce0fab6061733f7e1959c12b843079042fe13f56ca7b9d178c

ARG TARGETOS
ARG TARGETARCH

COPY bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
COPY crds /crds
COPY webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]
