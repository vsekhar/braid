# from: https://github.com/devcontainers/images/blob/main/src/go/.devcontainer/Dockerfile

FROM golang:1-bullseye

ENV DEBIAN_FRONTEND=noninteractive

# Google Cloud CLI repo
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.asc] https://packages.cloud.google.com/apt cloud-sdk main" | tee /etc/apt/sources.list.d/google-cloud-sdk.list \
    && curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | tee /usr/share/keyrings/cloud.google.asc

# Hashicorp Terraform repo
RUN echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com bullseye main" | tee /etc/apt/sources.list.d/hashicorp.list \
    && curl -sSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg

RUN apt-get update \
    && apt-get -y install \
        protobuf-compiler \
        google-cloud-sdk \
        terraform \
    && apt-get autoremove -y \
    && apt-get clean -y \
    && rm -rf /var/lib/apt/lists/*

# For protobuf
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

# For VS Code Go extension
RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@v1.21.1 \
    && go install honnef.co/go/tools/cmd/staticcheck@2023.1.6

ENV DEBIAN_FRONTEND=dialog
