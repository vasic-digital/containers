# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot

COPY bin/core /core

EXPOSE 8080 50051

ENTRYPOINT ["/core"]
