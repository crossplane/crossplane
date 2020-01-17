FROM ubuntu:18.04

WORKDIR /tmp

RUN apt-get update
RUN apt-get install -y curl

RUN curl -sL https://raw.githubusercontent.com/helm/helm/master/scripts/get > install-helm
RUN chmod +x install-helm
RUN ./install-helm --version v2.15.2
RUN helm init --client-only

ENTRYPOINT ["helm"]
