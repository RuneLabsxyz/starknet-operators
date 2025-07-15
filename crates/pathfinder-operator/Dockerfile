FROM rust:1.87-alpine AS builder
WORKDIR /usr/src/pathfinder-operator
RUN apk add --no-cache build-base
COPY ../../ ./
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    cargo install --path . pathfinder-operator

FROM cgr.dev/chainguard/static
LABEL org.opencontainers.image.source https://github.com/Runelabsxyz/deployment
COPY --from=builder /usr/local/cargo/bin/pathfinder-operator /app/pathfinder-operator
EXPOSE 8080
ENTRYPOINT ["/app/pathfinder-operator"]
