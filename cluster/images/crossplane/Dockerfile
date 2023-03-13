FROM gcr.io/distroless/static@sha256:f1e013b5fe376746acf4fe41377681babbcf638f4d805bf0ed2795f65587c7e5

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]