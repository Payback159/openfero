# build stage
# golang:1.22.2-bookworm
FROM golang@sha256:ab48cd7b8e2cffb6fa1199de232f61c76d3c33dc158be8a998c5407a8e5eb583 AS build-env

RUN echo "openfero:x:10001:10001:OpenFero user:/app:/sbin/nologin" >> /etc/passwd_single && \
    echo "openfero:x:10001:" >> /etc/group_single

# final stage
FROM scratch

WORKDIR /app

COPY openfero /app/
COPY web/ /app/web/
COPY --from=build-env /etc/passwd_single /etc/passwd
COPY --from=build-env /etc/group_single /etc/group
USER openfero

CMD ["/app/openfero"]
