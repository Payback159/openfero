# build stage
# golang:1.20.3-bullseye
FROM golang@sha256:3b2c96dbce8b90f538ae226c473b6e92d969f8e0feddf6e6824a6035978bd5d1 AS build-env

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
