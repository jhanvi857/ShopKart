FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY edge-gateway ./edge-gateway
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/edge-gateway ./edge-gateway

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/edge-gateway /app/edge-gateway
EXPOSE 9000
ENTRYPOINT ["/app/edge-gateway"]
