# build stage
# golang:1.22.2-bookworm
FROM golang@sha256:0f7691253e132744318ff4b02e0824fec9fa4f3b43b08afd1195599660d51521 AS build-env

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
