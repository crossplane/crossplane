FROM mikefarah/yq:4.34.2

COPY --chmod=+x writer.sh /bin
USER root

ENTRYPOINT ["/bin/writer.sh"]