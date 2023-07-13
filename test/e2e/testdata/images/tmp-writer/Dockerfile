FROM mikefarah/yq:4.34.1

COPY writer.sh /bin
USER root
RUN chmod +x /bin/writer.sh

ENTRYPOINT ["/bin/writer.sh"]