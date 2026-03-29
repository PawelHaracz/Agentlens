FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init mcp-server && go build -o mcp-server .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/mcp-server .
EXPOSE 9002
CMD ["./mcp-server"]
