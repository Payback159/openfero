# build stage
# golang:1.20.2-bullseye
FROM golang@sha256:89924bd0abc1001141e0415648d90914ebc9a9d60d4cbbc696ee53f1d1a9a136 AS build-env

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
