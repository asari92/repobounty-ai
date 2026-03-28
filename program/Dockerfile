# Dockerfile for Solana program
FROM rust:1.79-slim-bookworm AS builder

# Install Solana CLI
RUN apt-get update && apt-get install -y bash bzip2 curl && \
    sh -c "$(curl -sSfL https://release.anza.xyz/stable/install)"

ENV PATH="/root/.local/share/solana/install/active_release/bin:/root/.cargo/bin:${PATH}"

# Install the Anchor CLI version used by this repository.
RUN cargo install --locked anchor-cli --version 0.30.1 --force

WORKDIR /app

COPY . .
RUN anchor build --no-idl

FROM busybox:1.36.1-musl

WORKDIR /artifacts

COPY --from=builder /app/target/deploy ./target/deploy

# For now, keep only the built program artifacts in the final image.
CMD ["sh", "-c", "ls -lah /artifacts/target/deploy && echo Program built"]
