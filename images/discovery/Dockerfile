# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot

COPY bin/discovery /discovery

EXPOSE 50052

ENTRYPOINT ["/discovery"]
