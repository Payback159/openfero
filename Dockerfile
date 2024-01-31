# build stage
# golang:1.21.6-bookworm
FROM golang@sha256:d8c365d63879c2312e332cb796961f2695dd65124ceb3c0247d9c5426b7dde5f AS build-env

COPY certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates && \
    echo "openfero:x:10001:10001:OpenFero user:/app:/sbin/nologin" >> /etc/passwd_single && \
    echo "openfero:x:10001:" >> /etc/group_single

RUN mkdir /build
COPY . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o openfero .

# final stage
FROM scratch
WORKDIR /app
COPY --from=build-env /build/openfero /app/
COPY --from=build-env /etc/passwd_single /etc/passwd
COPY --from=build-env /etc/group_single /etc/group
USER openfero

CMD ["/app/openfero"]
