FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o rosecicd-builder ./cmd/rosecicd-builder

FROM alpine:3.21
RUN apk add --no-cache buildah git ca-certificates fuse-overlayfs
COPY --from=builder /build/rosecicd-builder /usr/local/bin/rosecicd-builder
ENTRYPOINT ["/usr/local/bin/rosecicd-builder"]
