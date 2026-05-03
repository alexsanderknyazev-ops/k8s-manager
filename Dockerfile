# Build stage — Go >= go.mod `go` directive (currently 1.25).
# GOTOOLCHAIN=auto: при GOTOOLCHAIN=local из окружения не остаёмся на старом toolchain в базовом образе.
FROM golang:1.25-alpine AS builder
WORKDIR /app
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
COPY vendor ./vendor
COPY *.go ./
COPY api ./api
COPY internal ./internal
COPY templates ./templates
COPY static ./static

RUN go version && CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o k8s-manager .

# Run stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /app/k8s-manager .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

EXPOSE 8080
ENV PORT=8080

ENTRYPOINT ["./k8s-manager"]
