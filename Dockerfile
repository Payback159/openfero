# build stage
# golang:1.22.1-bookworm
FROM golang@sha256:0b55ab82ac2a54a6f8f85ec8b943b9e470c39e32c109b766bbc1b801f3fa8d3b AS build-env

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
