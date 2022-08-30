# Build the manager binary
FROM golang:1.18 as builder

# Copy the contents of the repository
ADD . /workspace/go/src/github.com/csi-addons/kubernetes-csi-addons

ENV GOPATH=/workspace/go
WORKDIR /workspace/go/src/github.com/csi-addons/kubernetes-csi-addons

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -o /workspace/manager cmd/manager/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
