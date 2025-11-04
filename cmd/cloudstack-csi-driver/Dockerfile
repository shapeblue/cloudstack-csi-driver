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
    go build -ldflags "${LDFLAGS}" -o cloudstack-csi-driver ./cmd/cloudstack-csi-driver

FROM alpine:3.21

LABEL \
    org.opencontainers.image.description="CloudStack CSI driver" \
    org.opencontainers.image.source="https://github.com/cloudstack/cloudstack-csi-driver/"

RUN apk add --no-cache \
    ca-certificates \
    # Provides mkfs.ext2, mkfs.ext3, mkfs.ext4 (used by k8s.io/mount-utils)
    e2fsprogs \
    e2fsprogs-extra \
    # Provides mkfs.xfs
    xfsprogs \
    # blkid, mount and umount are required by k8s.io/mount-utils \
    blkid \
    mount \
    umount \
    # Provides udevadm for device path detection \
    udev

COPY --from=builder /workspace/cloudstack-csi-driver /cloudstack-csi-driver
ENTRYPOINT ["/cloudstack-csi-driver"]
