# syntax=docker/dockerfile:1
FROM ghcr.io/vasic-digital/android-emulator-base:latest

ENV ANDROID_SDK_ROOT=/opt/android-sdk
ENV EMULATOR_AVD_DIR=/root/.android/avd

RUN sdkmanager --install \
    "system-images;android-28;google_apis;x86_64" \
    "system-images;android-29;google_apis;x86_64" \
    "system-images;android-30;google_apis;x86_64" \
    "system-images;android-31;google_apis;x86_64" \
    "system-images;android-33;google_apis;x86_64" \
    "system-images;android-34;google_apis;x86_64" \
    "system-images;android-35;google_apis;x86_64" \
    "system-images;android-36;google_apis;x86_64"

COPY avds/ $EMULATOR_AVD_DIR/

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
