# build stage
# golang:1.20.4-bullseye
FROM golang@sha256:685a22e459f9516f27d975c5cc6accc11223ee81fdfbbae60e39cc3b87357306 AS build-env

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
