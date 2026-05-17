# Containerfile for the crossbuild Web/Wasm builder image.
# Provides JDK 21 + Gradle 8.11.1 in a slim Linux container suitable
# for running `wasmJsBrowserDistribution` (or any KGP-based Wasm
# build task) on any host OS with rootless podman or docker.
#
# Upstream constraint (documented honestly per CONST-039):
#   KGP <= 2.0.x exits 0 from wasmJsBrowserDistribution but does NOT
#   copy output to build/dist/wasmJs/productionExecutable/. This is
#   tracked as #wasmjs-production-distribution-gap. Upgrade the
#   consumer project's KGP to > 2.1.x to resolve. This image itself
#   is correct; the limitation is upstream KGP.
#
# Build command (any host with rootless podman/docker):
#
#   podman build \
#       -t ghcr.io/vasic-digital/crossbuild-web-wasm:jdk21 \
#       -f web_wasm.Containerfile \
#       pkg/crossbuild/
#
# Verify:
#
#   podman run --rm ghcr.io/vasic-digital/crossbuild-web-wasm:jdk21 \
#       gradle --version
#
# Once this image is on the host, WebWasmContainerBackend.Build()
# succeeds end-to-end (given consumer project KGP > 2.1.x).

ARG BASE_IMAGE=eclipse-temurin:21-jdk-noble
FROM ${BASE_IMAGE}

LABEL org.opencontainers.image.title="crossbuild-web-wasm"
LABEL org.opencontainers.image.description="JDK 21 + Gradle container for Kotlin/Wasm browser distribution builds"
LABEL org.opencontainers.image.source="https://github.com/vasic-digital/Containers"
LABEL org.opencontainers.image.licenses="See parent project LICENSE"
LABEL digital.vasic.containers.package="pkg/crossbuild"

ENV DEBIAN_FRONTEND=noninteractive
ENV PATH=/opt/gradle/bin:${PATH}

# Node.js is required by KGP's Wasm/JS toolchain at build time.
# Version 20 (LTS) is the minimum tested with KGP 2.0.x+.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg unzip zip \
 && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && rm -rf /var/lib/apt/lists/*

# Gradle 8.11.1 — pinned to match the minimum wrapper version
# supported by KGP 2.0.x+.
ENV GRADLE_VERSION=8.11.1
RUN curl -fsSL "https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip" \
        -o /tmp/gradle.zip \
 && unzip -q /tmp/gradle.zip -d /opt \
 && mv "/opt/gradle-${GRADLE_VERSION}" /opt/gradle \
 && rm /tmp/gradle.zip \
 && /opt/gradle/bin/gradle --version

# Pre-warm the Gradle daemon cache so the first consumer build is
# faster. The consumer project provides its own gradle wrapper; this
# is just the system Gradle.
RUN gradle --version

WORKDIR /work/src

# Default: report versions so the orchestrator can smoke-test the image
# quickly via `podman run --rm <image>`.
CMD ["/bin/sh", "-c", "java -version && node --version && gradle --version"]
