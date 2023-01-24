FROM gcr.io/distroless/static@sha256:5bda5a482de9b13160c061a3fd8665eecf8e35f5ed78e324dafdcdb11cf6d5da

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]