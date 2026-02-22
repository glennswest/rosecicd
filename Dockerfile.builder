FROM docker.io/library/alpine:3.21
RUN apk add --no-cache buildah git ca-certificates fuse-overlayfs
COPY bin/rosecicd-builder /usr/local/bin/rosecicd-builder
ENTRYPOINT ["/usr/local/bin/rosecicd-builder"]
