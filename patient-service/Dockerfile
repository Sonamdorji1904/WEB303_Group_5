FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /patient-service ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S patient && adduser -S patient -G patient
USER patient
WORKDIR /app
COPY --from=build /patient-service /app/patient-service
EXPOSE 8081
ENTRYPOINT ["/app/patient-service"]
