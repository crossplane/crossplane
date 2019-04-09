FROM ubuntu:16.04
RUN apt-get update
RUN ulimit -n 65536
RUN apt-get install -y curl
RUN curl https://packages.treasuredata.com/GPG-KEY-td-agent | apt-key add -
RUN echo "deb http://packages.treasuredata.com/3/ubuntu/xenial/ xenial contrib" > /etc/apt/sources.list.d/treasure-data.list
RUN apt-get update && apt-get install -y -q curl make g++ && apt-get clean && apt-get install -y td-agent && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN sed -i -e "s/USER=td-agent/USER=root/" -e "s/GROUP=td-agent/GROUP=root/" /etc/init.d/td-agent
CMD /usr/sbin/td-agent $FLUENTD_ARGS
