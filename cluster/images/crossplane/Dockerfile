FROM gcr.io/distroless/static@sha256:d2e099306ea61cb96017e227dc23efd64a844ccff439c095023e4983cb601f62

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]