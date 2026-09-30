# syntax=docker/dockerfile:1

FROM golang:1.27.1-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY internal ./internal

ARG TARGETOS=linux
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/wallpaper-api . \
    && mkdir -p /out/certs /out/cache

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/wallpaper-api /wallpaper-api
# Certificate cache and image cache directories, owned by the unprivileged user.
COPY --from=build --chown=65532:65532 /out/certs /data/certs
COPY --from=build --chown=65532:65532 /out/cache /data/cache

USER 65532:65532

ENV WALLPAPER_ADDR=:8080 \
    WALLPAPER_ACME_CACHE_DIR=/data/certs \
    WALLPAPER_CACHE_DIR=/data/cache

# 8080 serves IP/HTTP mode; 80 and 443 are used in domain/ACME mode.
EXPOSE 8080 80 443

ENTRYPOINT ["/wallpaper-api"]
