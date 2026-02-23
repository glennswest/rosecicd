FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o rosecicd ./cmd/rosecicd

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/rosecicd /usr/local/bin/rosecicd
COPY deploy/config.yaml /usr/share/rosecicd/config.yaml
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/rosecicd"]
