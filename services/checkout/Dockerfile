FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY checkout ./checkout
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/checkout ./checkout

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/checkout /app/checkout
EXPOSE 9003
ENTRYPOINT ["/app/checkout"]
