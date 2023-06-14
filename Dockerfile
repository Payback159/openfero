# build stage
# golang:1.20.4-bullseye
FROM golang@sha256:e7bb4d1d1afaf8358a17937da72922cbc043e61c60d2b82f25245d46240e231a AS build-env

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
