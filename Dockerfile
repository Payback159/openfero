# build stage
# golang:1.20.1-bullseye
FROM golang@sha256:51ff22f03320894402290ba7dfd83ee05b61e58b5381d76b40f2e3a370d81da3 AS build-env

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
