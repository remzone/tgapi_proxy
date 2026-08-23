ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
ENV GOMAXPROCS=1 \
    GOFLAGS=-p=1
ARG MTG_VERSION=v2.2.8
RUN GOBIN=/out go install github.com/9seconds/mtg/v2@${MTG_VERSION}

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tgproxy ./cmd/tgproxy

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && addgroup -S tgproxy && adduser -S -G tgproxy tgproxy
COPY --from=build /out/tgproxy /usr/local/bin/tgproxy
COPY --from=build /out/mtg /usr/local/bin/mtg
RUN mkdir -p /var/log/tgproxy && chown tgproxy:tgproxy /var/log/tgproxy
USER tgproxy
EXPOSE 443 8080
ENTRYPOINT ["tgproxy"]
CMD ["serve-all"]
