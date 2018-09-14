FROM BASEIMAGE
RUN apk --no-cache add ca-certificates bash

ARG ARCH
ARG TINI_VERSION

ADD conductor /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["conductor"]
CMD ["--install-crds=false"]