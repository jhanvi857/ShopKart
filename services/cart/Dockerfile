FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY cart ./cart
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/cart ./cart

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/cart /app/cart
EXPOSE 9006
ENTRYPOINT ["/app/cart"]
