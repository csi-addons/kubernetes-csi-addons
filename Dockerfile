# Build the manager binary
FROM golang:1.21 as builder

# Copy the contents of the repository
COPY . /workspace/go/src/github.com/csi-addons/kubernetes-csi-addons

ENV GOPATH=/workspace/go CGO_ENABLED=0
WORKDIR /workspace/go/src/github.com/csi-addons/kubernetes-csi-addons

# Build
RUN make build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/go/src/github.com/csi-addons/kubernetes-csi-addons/bin/csi-addons-manager .
USER 65532:65532

ENTRYPOINT ["/csi-addons-manager"]
