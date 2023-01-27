# from: https://github.com/devcontainers/images/blob/main/src/go/.devcontainer/Dockerfile

FROM golang:1-bullseye

# Install protoc, go protoc/grpc plugins, terraform
# (derived from: https://github.com/dev-container/cloud/blob/master/.devcontainer/Dockerfile)

RUN \
    && curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | tee /usr/share/keyrings/cloud.google.gpg \
    && curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor | tee /usr/share/keyrings/hashicorp-archive-keyring.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list \
    && echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list \
    && apt-get update \
    && apt-get -y install \
        protobuf-compiler \
        google-cloud-sdk \
        terraform \
    && apt-get autoremove -y \
    && apt-get clean -y \
    && rm -rf /var/lib/apt/lists/*

RUN go install \
    google.golang.org/protobuf/cmd/protoc-gen-go@v1.28 \
    google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
