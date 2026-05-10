# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.24.6-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY app/go.mod app/go.sum* ./app/
WORKDIR /src/app
ENV CGO_ENABLED=0
RUN apk add --no-cache ca-certificates tzdata
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY app/ /src/app/
RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/tgtldr ./cmd/server

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/tgtldr /app/tgtldr
ENV TGTLDR_HTTP_ADDR=:8080
EXPOSE 8080
CMD ["/app/tgtldr"]
