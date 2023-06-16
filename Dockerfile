# build stage
# golang:1.20.4-bullseye
FROM golang@sha256:6b3fa4b908676231b50acbbc00e84d8cee9c6ce072b1175c0ff352c57d8a612f AS build-env

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
