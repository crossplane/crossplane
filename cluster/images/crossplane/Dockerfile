# TODO(negz): Fetch this outside of Docker? That way we can cache it somewhere
# (_output?) and not pull it on every build.
FROM curlimages/curl@sha256:c1c1cda72ab8c306390fc05518bf4d42148564978326d078f65d546858d139cb as curl

ARG TARGETOS
ARG TARGETARCH
ARG CRUN_VERSION=1.4.4  # TODO(negz): Plumb this up to the Makefile

WORKDIR /curl
RUN curl -fsSL https://github.com/containers/crun/releases/download/${CRUN_VERSION}/crun-${CRUN_VERSION}-${TARGETOS}-${TARGETARCH} -o crun
RUN chmod +x crun

FROM gcr.io/distroless/base@sha256:ce8bc342dd7eeb0baccbef2ce00afc0c72af1ea166794f55ef8f434fd7c8b515

ARG TARGETOS
ARG TARGETARCH

COPY --from=curl /curl/crun /usr/local/bin/
ADD bin/${TARGETOS}\_${TARGETARCH}/crossplane /usr/local/bin/
ADD crds /crds
ADD webhookconfigurations /webhookconfigurations
EXPOSE 8080
USER 65532
ENTRYPOINT ["crossplane"]
