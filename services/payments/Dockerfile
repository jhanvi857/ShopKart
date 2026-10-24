FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY payments ./payments
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/payments ./payments

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/payments /app/payments
EXPOSE 9005
ENTRYPOINT ["/app/payments"]
