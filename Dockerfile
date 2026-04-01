# Stage 1: Build frontend
FROM oven/bun:1.3.11-alpine AS frontend
WORKDIR /web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# Stage 2: Build Go binary
FROM golang:1.26.1-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /web/dist ./web/dist
RUN CGO_ENABLED=1 go build -o agentlens ./cmd/agentlens

# Stage 3: Runtime
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/agentlens .
EXPOSE 8080
CMD ["./agentlens"]
