FROM alpine:3.18

LABEL \
    org.opencontainers.image.description="CloudStack disk offering to Kubernetes storage class syncer" \
    org.opencontainers.image.source="https://github.com/shapeblue/cloudstack-csi-driver/"

RUN apk add --no-cache ca-certificates

COPY ./bin/cloudstack-csi-sc-syncer /cloudstack-csi-sc-syncer
ENTRYPOINT ["/cloudstack-csi-sc-syncer"]