FROM golang:1.22-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/berkut-scc ./main.go

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata pandoc libreoffice && \
    addgroup -S berkut && adduser -S -G berkut berkut && \
    mkdir -p /app/config /app/data && chown -R berkut:berkut /app

COPY --from=builder /out/berkut-scc /usr/local/bin/berkut-scc
COPY config/app.yaml /app/config/app.yaml

USER berkut

ENV APP_CONFIG=/app/config/app.yaml \
    ENV=prod \
    APP_ENV=prod \
    PORT=8080 \
    DATA_PATH=/app/data \
    BERKUT_LISTEN_ADDR=0.0.0.0:8080 \
    BERKUT_DB_PATH=/app/data/berkut.db \
    BERKUT_DOCS_STORAGE_DIR=/app/data/docs \
    BERKUT_INCIDENTS_STORAGE_DIR=/app/data/incidents

VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/berkut-scc"]
