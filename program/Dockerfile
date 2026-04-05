FROM node:20-bookworm-slim AS node-runtime

FROM rust:1.85.0-slim-bookworm

ARG SOLANA_RUNTIME_VERSION=v2.1.21
ARG SOLANA_BUILD_VERSION=v3.1.11

COPY --from=node-runtime /usr/local /usr/local

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    build-essential \
    bzip2 \
    ca-certificates \
    curl \
    libssl-dev \
    libudev-dev \
    perl \
    perl-modules \
    pkg-config \
    xz-utils && \
    rm -rf /var/lib/apt/lists/*

RUN arch="$(dpkg --print-architecture)" && \
    case "$arch" in \
      amd64) agave_arch="x86_64" ;; \
      arm64) agave_arch="aarch64" ;; \
      *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac && \
    target="${agave_arch}-unknown-linux-gnu" && \
    runtime_url="https://github.com/anza-xyz/agave/releases/download/${SOLANA_RUNTIME_VERSION}/solana-release-${target}.tar.bz2" && \
    build_url="https://github.com/anza-xyz/agave/releases/download/${SOLANA_BUILD_VERSION}/solana-release-${target}.tar.bz2" && \
    echo "Downloading Solana runtime from ${runtime_url}" && \
    curl --http1.1 -fL "${runtime_url}" -o /tmp/solana-runtime.tar.bz2 && \
    mkdir -p /opt/solana-runtime && \
    tar -xjf /tmp/solana-runtime.tar.bz2 --strip-components=1 -C /opt/solana-runtime && \
    echo "Downloading Solana build tools from ${build_url}" && \
    curl --http1.1 -fL "${build_url}" -o /tmp/solana-build.tar.bz2 && \
    mkdir -p /opt/solana-build && \
    tar -xjf /tmp/solana-build.tar.bz2 --strip-components=1 -C /opt/solana-build && \
    rm -f /tmp/solana-runtime.tar.bz2 /tmp/solana-build.tar.bz2

RUN cargo install --locked anchor-cli --version 0.32.1 --force

RUN /opt/solana-build/bin/platform-tools-sdk/sbf/scripts/install.sh

RUN ln -sf /opt/solana-build/bin/cargo-build-sbf /usr/local/bin/cargo-build-sbf && \
    ln -sf /opt/solana-build/bin/cargo-test-sbf /usr/local/bin/cargo-test-sbf

ENV PATH="/usr/local/bin:/opt/solana-runtime/bin:/usr/local/cargo/bin:${PATH}"

WORKDIR /app

CMD ["sh", "-lc", "anchor --version && solana --version && rustc --version && cargo --version"]
