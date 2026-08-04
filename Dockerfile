# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.5

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG REVISION=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w -X main.releaseVersion=${VERSION} -X main.sourceRevision=${REVISION}" \
      -o /out/koment ./cmd/koment

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION
ARG REVISION
LABEL org.opencontainers.image.title="koment" \
      org.opencontainers.image.description="Out-of-band code annotations, checked against the code they describe" \
      org.opencontainers.image.source="https://github.com/janpuc/koment" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      io.modelcontextprotocol.server.name="io.github.janpuc/koment"

COPY --from=build /out/koment /usr/local/bin/koment

WORKDIR /
USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/koment"]
CMD ["serve", "--config", "/config/repositories.yaml", "--listen", "0.0.0.0:8080"]
