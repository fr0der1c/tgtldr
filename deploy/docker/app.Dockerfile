FROM golang:1.24.6-alpine AS builder

WORKDIR /src
COPY app/go.mod app/go.sum* ./app/
WORKDIR /src/app
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY app/ /src/app/
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -o /out/tgtldr ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/tgtldr /app/tgtldr
ENV TGTLDR_HTTP_ADDR=:8080
EXPOSE 8080
CMD ["/app/tgtldr"]
