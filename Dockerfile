# syntax=docker/dockerfile:1.7
# ──────────────────────────────────────────────────────────────────────────────
# Stage 1: Build
# ──────────────────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Cache dependencies separately from source code
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /manager ./cmd/main.go

# ──────────────────────────────────────────────────────────────────────────────
# Stage 2: Runtime (distroless — no shell, minimal attack surface)
# ──────────────────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
