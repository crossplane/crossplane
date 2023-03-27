FROM gcr.io/distroless/static@sha256:97b762efb017cbbabf566046852de8049f84f73e168282d06da316851c7ef263

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]