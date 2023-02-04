FROM gcr.io/distroless/static@sha256:fc8cbb80b6a489b662c0a208caaef5c10b2094d07291029337fe9010380b4aac

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]