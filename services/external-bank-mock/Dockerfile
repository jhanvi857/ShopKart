FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY external-bank-mock ./external-bank-mock
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/external-bank-mock ./external-bank-mock

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/external-bank-mock /app/external-bank-mock
EXPOSE 9008
ENTRYPOINT ["/app/external-bank-mock"]
