FROM gcr.io/distroless/static@sha256:c79ef2d19cf18345ffef734a4ef5344bf7b355bb335eca38740a096bde891521

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]