# build stage
# golang:1.20.4-bullseye
FROM golang@sha256:1a03e9e3cdc761ca91f5883dee232b1cb5c76514fc16581ec5b11991c5f3fb1d AS build-env

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
