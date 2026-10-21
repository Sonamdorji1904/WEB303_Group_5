FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /doctor-service ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S doctor && adduser -S doctor -G doctor
USER doctor
WORKDIR /app
COPY --from=build /doctor-service /app/doctor-service
EXPOSE 8082
ENTRYPOINT ["/app/doctor-service"]
