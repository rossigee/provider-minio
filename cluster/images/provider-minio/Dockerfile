FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY provider-minio /usr/bin/provider-minio
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/usr/bin/provider-minio", "operator"]
