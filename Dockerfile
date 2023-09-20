# build stage
# golang:1.21.1-bookworm
FROM golang@sha256:d3114db136e7f2d9b08c3fcd374381ba3f8d85c17b80c48dbc53cd938cfb2b64 AS build-env

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
