FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY notifications ./notifications
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/notifications ./notifications

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/notifications /app/notifications
EXPOSE 9007
ENTRYPOINT ["/app/notifications"]
