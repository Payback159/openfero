# build stage
# golang:1.20.1-bullseye
FROM golang@sha256:c3fbdc381fb6b78325c2a5cc1bf0c288c0d173568fba3f1b8894a51837cccf7f AS build-env

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
