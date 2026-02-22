FROM docker.io/library/alpine:3.21
RUN apk add --no-cache ca-certificates
COPY bin/rosecicd /usr/local/bin/rosecicd
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/rosecicd"]
