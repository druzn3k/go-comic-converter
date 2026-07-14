# =============================================================================
# Stage 1: Build the Go binary
# =============================================================================
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Dependency layer — cached unless go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

# Source layer — invalidated on any source change
COPY . .

# Static binary, stripped
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w" \
    -o /go-comic-converter \
    .

# =============================================================================
# Stage 2: Minimal runtime image
# =============================================================================
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /go-comic-converter /usr/local/bin/go-comic-converter

# Working directory for mounted comic volumes
WORKDIR /data

# HTTP server port
EXPOSE 8080

ENTRYPOINT ["go-comic-converter"]
CMD ["--help"]
