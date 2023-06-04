FROM --platform=$BUILDPLATFORM golang:1.20 as builder

WORKDIR /workspace

ENV CGO_ENABLED=0
COPY go.mod go.sum ./

RUN go mod download

COPY . .
RUN go build -trimpath -ldflags '-s -w' -o /out/controller main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /out/controller .

USER 65532:65532

ENTRYPOINT ["/controller"]
