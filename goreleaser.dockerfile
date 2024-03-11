# build stage
# golang:1.22.1-bookworm
FROM golang@sha256:34ce21a9696a017249614876638ea37ceca13cdd88f582caad06f87a8aa45bf3 AS build-env

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
