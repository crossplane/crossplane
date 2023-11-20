FROM mikefarah/yq:4.40.2

COPY --chmod=+x writer.sh /bin
USER root

ENTRYPOINT ["/bin/writer.sh"]