# build stage
FROM golang@sha256:86637db6bba97ec488c2f9f299f18284cd45390de2932641ad706a758e9c36e7 AS build-env # golang:1.20.3-bullseye

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
