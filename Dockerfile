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
    && mkdir -p /out/cache

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/wallpaper-api /wallpaper-api
# Image cache directory, owned by the unprivileged user.
COPY --from=build --chown=65532:65532 /out/cache /data/cache

USER 65532:65532

ENV WALLPAPER_ADDR=:8080 \
    WALLPAPER_CACHE_DIR=/data/cache

# The service speaks plain HTTP; TLS is terminated by the caddy container.
EXPOSE 8080

ENTRYPOINT ["/wallpaper-api"]
