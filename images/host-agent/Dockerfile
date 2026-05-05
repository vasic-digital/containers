# syntax=docker/dockerfile:1
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    libnvidia-encode-550 \
    libva-drm2 \
    libva-x11-2 \
    mesa-vulkan-drivers \
    vulkan-tools \
    && rm -rf /var/lib/apt/lists/*

COPY bin/host-agent /host-agent

EXPOSE 47998-48000/udp

ENTRYPOINT ["/host-agent"]
