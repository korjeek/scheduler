FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /out/scheduler \
    ./cmd/scheduler

FROM alpine:3.20
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/scheduler .
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8080 50051 9090
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:9090/healthz || exit 1
ENTRYPOINT ["./scheduler"]