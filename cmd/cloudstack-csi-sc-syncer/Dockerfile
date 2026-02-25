FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG LDFLAGS

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "${LDFLAGS}" -o cloudstack-csi-sc-syncer ./cmd/cloudstack-csi-sc-syncer

FROM alpine:3.23

LABEL \
    org.opencontainers.image.description="CloudStack disk offering to Kubernetes storage class syncer" \
    org.opencontainers.image.source="https://github.com/cloudstack/cloudstack-csi-driver/"

RUN apk add --no-cache ca-certificates

COPY --from=builder /workspace/cloudstack-csi-sc-syncer /cloudstack-csi-sc-syncer
ENTRYPOINT ["/cloudstack-csi-sc-syncer"]