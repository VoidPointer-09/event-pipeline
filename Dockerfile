# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/producer ./cmd/producer && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/consumer ./cmd/consumer && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./cmd/api

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/producer /app/producer
COPY --from=build /out/consumer /app/consumer
COPY --from=build /out/api /app/api
USER nonroot:nonroot

