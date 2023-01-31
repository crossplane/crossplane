FROM gcr.io/distroless/static@sha256:450981e797a3da56ef2edf42691fc21db464c2bb351e60bcccb08aed2d49ef83

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]
