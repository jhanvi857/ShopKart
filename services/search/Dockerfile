FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY search ./search
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/search ./search

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/search /app/search
EXPOSE 9002
ENTRYPOINT ["/app/search"]
