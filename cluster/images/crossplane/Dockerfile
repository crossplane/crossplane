FROM BASEIMAGE
RUN apk --no-cache add ca-certificates bash

ARG ARCH
ARG TINI_VERSION

ADD crossplane /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["crossplane"]
CMD ["--install-crds=false"]