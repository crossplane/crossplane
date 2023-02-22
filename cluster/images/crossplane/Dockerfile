FROM gcr.io/distroless/static@sha256:d02be0e44ff47558d192c7fd475601ab65d9b0936f2da3e91bb6071819608689

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]