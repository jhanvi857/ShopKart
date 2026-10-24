FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY inventory ./inventory
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/inventory ./inventory

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/inventory /app/inventory
EXPOSE 9004
ENTRYPOINT ["/app/inventory"]
