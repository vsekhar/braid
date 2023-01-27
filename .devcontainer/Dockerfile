# from: https://github.com/devcontainers/images/blob/main/src/go/.devcontainer/Dockerfile

FROM golang:1-bullseye

# Install protoc and go protoc/grpc plugins
RUN apt-get update && apt-get install -y \
    protoc \
    && rm -rf /var/lib/apt/lists/*
# RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
# RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2