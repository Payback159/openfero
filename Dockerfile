# build stage
# golang:1.20.4-bullseye
FROM golang@sha256:4b1fc02d16fca272e5e6e6adc98396219b43ef663a377eef4a97e881d364393f AS build-env

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
