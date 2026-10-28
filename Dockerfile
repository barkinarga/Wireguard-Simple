FROM node:24-alpine AS ui
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY ui ./ui
COPY scripts/build.mjs ./scripts/build.mjs
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=ui /src/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /wgsimple ./cmd/wgsimple

FROM alpine:3
RUN apk add --no-cache wireguard-tools iproute2 bash ca-certificates
COPY --from=build /wgsimple /usr/local/bin/wgsimple
RUN mkdir -p /var/lib/wgsimple /etc/wireguard && chmod 700 /var/lib/wgsimple /etc/wireguard
ENV GOMEMLIMIT=128MiB
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/wgsimple"]
CMD ["--mode", "live", "--listen", "127.0.0.1:8787", "--data-dir", "/var/lib/wgsimple"]
