FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY reviews ./reviews
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/reviews ./reviews

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/reviews /app/reviews
EXPOSE 9009
ENTRYPOINT ["/app/reviews"]
