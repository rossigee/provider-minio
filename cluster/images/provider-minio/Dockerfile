FROM gcr.io/distroless/static:nonroot

# OpenContainers Image Spec Labels
LABEL org.opencontainers.image.title="provider-minio"
LABEL org.opencontainers.image.description="Crossplane provider for MinIO object storage (buckets, users, policies)"
LABEL org.opencontainers.image.vendor="rossigee"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/rossigee/provider-minio"
LABEL org.opencontainers.image.url="https://github.com/rossigee/provider-minio"
LABEL org.opencontainers.image.documentation="https://github.com/rossigee/provider-minio/blob/master/README.md"

ARG TARGETOS
ARG TARGETARCH
ARG VERSION

COPY --chmod=0755 bin/${TARGETOS}_${TARGETARCH}/provider /usr/local/bin/provider
COPY --chmod=0644 crossplane.yaml /package.yaml
COPY --chmod=0644 crds /crds
COPY --chmod=0644 webhook /webhook

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/provider"]
