# Build the manager binary
FROM golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/github.com/upbound/conductor
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/upbound/conductor/cmd/manager

# Copy the controller-manager into a thin image
FROM alpine:3.8
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /root/
COPY --from=builder /go/src/github.com/upbound/conductor/manager .
ENTRYPOINT ["./manager"]
