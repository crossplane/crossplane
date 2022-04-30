FROM alpine@sha256:4edbd2beb5f78b1014028f4fbb99f3237d9561100b6881aabbf5acce2c4f9454

ARG TARGETOS
ARG TARGETARCH

# TODO(negz): For debugging - remove me.
RUN apk add --no-cache skopeo umoci neovim bash

RUN apk add --no-cache ca-certificates crun
ADD bin/${TARGETOS}\_${TARGETARCH}/crossplane /usr/local/bin/
ADD bin/${TARGETOS}\_${TARGETARCH}/ignition /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
RUN adduser -u 65532 nonroot -D
USER 65532
ENTRYPOINT ["crossplane"]
