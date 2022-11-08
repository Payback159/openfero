# build stage
FROM golang@sha256:7ffa70183b7596e6bc1b78c132dbba9a6e05a26cd30eaa9832fecad64b83f029 AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o openfero .

# final stage
FROM scratch
WORKDIR /app
COPY --from=build-env /build/openfero /app/

CMD ["/app/openfero"]
