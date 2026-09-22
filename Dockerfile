# Build the manager binary
#
# Pin the builder to the build host platform ($BUILDPLATFORM) and cross-compile
# for the target platform. quay.io/projectquay/golang does not publish an
# arm/v7 variant, so building it natively (under emulation) is not possible.
# CGO is disabled, so the Go toolchain can cross-compile to every target.
FROM --platform=$BUILDPLATFORM quay.io/projectquay/golang:1.26 AS builder

# Populated automatically by buildx from the requested target platform.
ARG TARGETOS TARGETARCH TARGETVARIANT

# Copy the contents of the repository
COPY . /workspace/go/src/github.com/csi-addons/kubernetes-csi-addons

ENV GOPATH=/workspace/go CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH
WORKDIR /workspace/go/src/github.com/csi-addons/kubernetes-csi-addons

# Build only the manager binary. Set GOARM=7 for the arm/v7 target.
RUN if [ "$TARGETVARIANT" = "v7" ]; then export GOARM=7; fi; make build-manager

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/go/src/github.com/csi-addons/kubernetes-csi-addons/bin/csi-addons-manager .
USER 65532:65532

ENTRYPOINT ["/csi-addons-manager"]
