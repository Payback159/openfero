# build stage
# golang:1.22.1-bookworm
FROM golang@sha256:99c5da0eb0ffe1a01e46f54c3a6783df4d8a49bd3c16fbb3ac74c6e711541a93 AS build-env

RUN echo "openfero:x:10001:10001:OpenFero user:/app:/sbin/nologin" >> /etc/passwd_single && \
    echo "openfero:x:10001:" >> /etc/group_single

# final stage
FROM scratch
WORKDIR /app
COPY openfero /app/
COPY --from=build-env /etc/passwd_single /etc/passwd
COPY --from=build-env /etc/group_single /etc/group
USER openfero

CMD ["/app/openfero"]
