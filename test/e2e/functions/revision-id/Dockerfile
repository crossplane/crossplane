# Runtime image for e2e-function-revision-id.
#
# IDENTITY is baked into the binary so each published tag reports a distinct
# value. Build v1 with --build-arg IDENTITY=revision-one and v2 with
# --build-arg IDENTITY=revision-two (see README.md).
FROM golang:1.26 AS build

ARG IDENTITY=unknown
WORKDIR /fn

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.Identity=${IDENTITY}" -o /function .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /function /function
EXPOSE 9443
USER nonroot:nonroot
ENTRYPOINT ["/function"]
