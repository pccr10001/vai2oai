# syntax=docker/dockerfile:1

FROM golang:1.23.2-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/vai2oai .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /out/vai2oai /app/vai2oai

RUN apk add --no-cache ca-certificates

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/vai2oai"]
