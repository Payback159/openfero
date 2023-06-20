# build stage
# golang:1.20.5-bullseye
FROM golang@sha256:a3598b93d32819f1759893c532fa186bc61d58f1ced9aa49c2c77fe13383159a AS build-env

COPY certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
COPY . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o openfero .

# final stage
FROM scratch
WORKDIR /app
COPY --from=build-env /build/openfero /app/

CMD ["/app/openfero"]
