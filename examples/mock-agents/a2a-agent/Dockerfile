FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init a2a-agent && go build -o a2a-agent .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/a2a-agent .
EXPOSE 9001
CMD ["./a2a-agent"]
