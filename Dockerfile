# Multi-stage build for lab-backend
FROM golang:1.25.1-alpine AS builder

RUN apk add --no-cache make git ca-certificates

WORKDIR /build

COPY . .

# Build arguments for version info
ARG VERSION=dev
ARG GIT_COMMIT=dev

# Build the binary
RUN mkdir -p bin && \
    go build -ldflags="-w -s" \
    -o bin/ethereum-address-metrics-exporter .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bin/ethereum-address-metrics-exporter .

RUN chown -R appuser:appuser /app

USER appuser

# Run the binary
# Configuration should be provided via Kubernetes ConfigMap mounted as volume
ENTRYPOINT ["/app/ethereum-address-metrics-exporter"]
