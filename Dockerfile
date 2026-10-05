FROM node:22-alpine AS frontend

WORKDIR /src
COPY gui/frontend/package.json gui/frontend/package-lock.json ./
RUN npm ci
COPY gui/frontend/ ./
RUN npm run build

FROM golang:1.25.5-alpine AS builder

ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/dist/ ./internal/webassets/dist/
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /praetor-web \
    ./cmd/praetor-web/

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl \
    && addgroup -S -g 10001 praetor \
    && adduser -S -D -H -h /data -u 10001 -G praetor praetor \
    && mkdir -p /data/config /data/data /data/state /data/scripts /data/logs \
    && chown -R praetor:praetor /data

COPY --from=builder /praetor-web /usr/local/bin/praetor-web
COPY --from=builder /src/LICENSE /usr/share/licenses/praetor/LICENSE
COPY --chown=praetor:praetor packaging/docker/config.example.yaml /data/config/config.yaml

ENV HOME=/data \
    PRAETOR_CONFIG_DIR=/data/config \
    PRAETOR_DATA_DIR=/data/data \
    PRAETOR_STATE_DIR=/data/state

VOLUME ["/data"]
USER 10001:10001
EXPOSE 8787

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl --fail --silent --insecure https://127.0.0.1:8787/healthz >/dev/null

ENTRYPOINT ["/usr/local/bin/praetor-web"]
CMD ["--listen", "0.0.0.0:8787"]
