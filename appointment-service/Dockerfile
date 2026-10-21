FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /appointment-service ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S appointment && adduser -S appointment -G appointment
USER appointment
WORKDIR /app
COPY --from=build /appointment-service /app/appointment-service
EXPOSE 8083
ENTRYPOINT ["/app/appointment-service"]
