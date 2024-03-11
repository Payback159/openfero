# build stage
# golang:1.22.1-bookworm
FROM golang@sha256:6699d2852712f090399ccd4e8dfd079b4d55376f3ab3aff5b2dc8b7b1c11e27e AS build-env

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
