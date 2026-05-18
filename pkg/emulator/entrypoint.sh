#!/usr/bin/env bash
# Containerfile entrypoint for the §6.X Android emulator image.
# PID 1 inside the container.
#
# Inputs (via env):
#   ANDROID_AVD_NAME  — AVD identifier set by Containerized.Boot
#                       (default: "default", the image's pre-baked AVD)
#   ANDROID_COLD_BOOT — "true" or "false" (§6.I clause 6 gating runs
#                       MUST be cold-boot; default: true)
#
# Anti-bluff posture: the emulator process IS PID 1. When the emulator
# exits (e.g. boot failure), the container exits, and `podman rm -f`
# in Containerized.Teardown sees "container already gone" rather than
# a stuck process. This eliminates the §6.B class of bluff where the
# container reports "Up" while the emulator inside is crash-looping.

set -euo pipefail

AVD_NAME="${ANDROID_AVD_NAME:-default}"
COLD_BOOT_FLAG=""
if [[ "${ANDROID_COLD_BOOT:-true}" == "true" ]]; then
    COLD_BOOT_FLAG="-no-snapshot"
fi

# Sanity-check the AVD exists in the image. Avd-not-found errors at
# `emulator -avd` time produce opaque exit codes; explicit pre-check
# gives a diagnostic message that surfaces in `podman logs`.
if ! avdmanager list avd 2>/dev/null | grep -q "Name: ${AVD_NAME}"; then
    echo "ERROR: AVD '${AVD_NAME}' not found in image. Available:" >&2
    avdmanager list avd 2>&1 >&2 || true
    exit 1
fi

echo "[§6.X-entrypoint] booting emulator avd=${AVD_NAME} cold-boot=${ANDROID_COLD_BOOT:-true}" >&2

# Start the emulator. -no-window for headless, -no-audio for the same
# reason, -no-boot-anim for boot speed. -gpu swiftshader_indirect uses
# the software renderer (the only choice without host GPU passthrough
# inside containers in the general case).
#
# adb listens on 0.0.0.0:5555 inside the container so host-side adb
# can reach it via the port forward from Containerized.Boot. The
# default is loopback-only, which would make the forwarded port
# unreachable.
exec emulator -avd "${AVD_NAME}" \
    -no-window \
    -no-audio \
    -no-boot-anim \
    -gpu swiftshader_indirect \
    -port 5554 \
    -read-only \
    ${COLD_BOOT_FLAG}
