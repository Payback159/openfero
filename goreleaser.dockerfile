# build stage
# golang:1.22.2-bookworm
FROM golang@sha256:6f1a37176cde35c5a64b1835c7b303635c07e3d0348abc11f744e0ef1bcd8b60 AS build-env

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
