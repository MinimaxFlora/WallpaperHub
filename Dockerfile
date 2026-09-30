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
    && mkdir -p /out/certs /out/images

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/wallpaper-api /wallpaper-api
# Certificate cache and default image dir, owned by the unprivileged user.
COPY --from=build --chown=65532:65532 /out/certs /data/certs
COPY --from=build --chown=65532:65532 /out/images /data/images

USER 65532:65532

ENV WALLPAPER_ADDR=:8080 \
    WALLPAPER_IMAGES_DIR=/data/images \
    WALLPAPER_ACME_CACHE_DIR=/data/certs

EXPOSE 8080 80 443

ENTRYPOINT ["/wallpaper-api"]
