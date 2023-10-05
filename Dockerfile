# build env
FROM golang:1.21 AS build-env
COPY go.mod go.sum /src/
WORKDIR /src
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG release=
RUN <<EOR
  VERSION=$(git rev-parse --short HEAD)
  BUILDTIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
  RELEASE=$release
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/blob-sender -ldflags="-s -w -X 'github.com/ethpandaops/blob-spammer/utils.BuildVersion=${VERSION}' -X 'github.com/ethpandaops/blob-spammer/utils.BuildRelease=${RELEASE}' -X 'github.com/ethpandaops/blob-spammer/utils.Buildtime=${BUILDTIME}'" ./cmd/blob-sender
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/blob-tester -ldflags="-s -w -X 'github.com/ethpandaops/blob-spammer/utils.BuildVersion=${VERSION}' -X 'github.com/ethpandaops/blob-spammer/utils.BuildRelease=${RELEASE}' -X 'github.com/ethpandaops/blob-spammer/utils.Buildtime=${BUILDTIME}'" ./cmd/blob-spammer
EOR

# final stage
FROM debian:stable-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates
ENV PATH="$PATH:/app"
COPY --from=build-env /app/* /app
CMD ["./blob-spammer"]
