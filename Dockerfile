# Stage 1: Build frontend
FROM oven/bun:1.3.11-alpine AS frontend
WORKDIR /web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# Stage 2: Build Go binary
FROM golang:1.26.1 AS builder
ARG VERSION=dev
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /web/dist ./web/dist
RUN CGO_ENABLED=1 go build -ldflags "-X main.version=${VERSION}" -o agentlens ./cmd/agentlens

# Stage 3: Distroless runtime (nonroot — UID 65532, matches Helm podSecurityContext)
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=builder /app/agentlens .
USER 65532:65532
EXPOSE 8080
CMD ["./agentlens"]
