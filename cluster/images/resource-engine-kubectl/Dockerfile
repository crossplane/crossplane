FROM ubuntu:18.04

WORKDIR /tmp

RUN apt-get update
RUN apt-get install -y curl

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.16.2/bin/linux/amd64/kubectl
RUN chmod +x kubectl && mv kubectl /usr/local/bin/kubectl

ENTRYPOINT ["kubectl"]
