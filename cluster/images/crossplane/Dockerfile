FROM BASEIMAGE

ADD crossplane /usr/local/bin/
ADD crds /crds
EXPOSE 8080
USER 1001
ENTRYPOINT ["crossplane"]
