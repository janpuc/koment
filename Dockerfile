# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG REVISION=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# CGO stays off so the result is a static binary the distroless base can run.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.revision=${REVISION}" \
      -o /out/koment ./cmd/koment

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION
ARG REVISION
LABEL org.opencontainers.image.title="koment" \
      org.opencontainers.image.description="Out-of-band code annotations, checked against the code they describe" \
      org.opencontainers.image.source="https://github.com/janpuc/koment" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

COPY --from=build /out/koment /usr/local/bin/koment

# The repository is mounted, not baked in: an image tied to one checkout would
# be a different image per repository.
WORKDIR /repo
USER nonroot:nonroot
EXPOSE 8080

# Read-only by default. ADR 0011 makes the unauthenticated posture conditional
# on that, so the image must not default to anything that writes.
ENTRYPOINT ["/usr/local/bin/koment"]
CMD ["ui", "--listen", "0.0.0.0:8080"]
