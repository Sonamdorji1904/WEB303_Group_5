# ---- build ----
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/notification-service ./cmd/server

# ---- run ----
FROM alpine:3.20
RUN adduser -D -u 10001 app && apk add --no-cache ca-certificates wget
USER app
COPY --from=build /out/notification-service /usr/local/bin/notification-service
EXPOSE 8084
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -qO- http://localhost:8084/health >/dev/null 2>&1 || exit 1
ENTRYPOINT ["notification-service"]
