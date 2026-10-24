FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY pkg ./pkg
COPY catalog ./catalog
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/catalog ./catalog

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /bin/catalog /app/catalog
EXPOSE 9001
ENTRYPOINT ["/app/catalog"]
