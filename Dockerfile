# build stage
# golang:1.19.4-bullseye
FROM golang@sha256:ab3bac67a27861e5ffcf4efe4ecabec1a01734db47cc0e85eeadde17856c52ac AS build-env

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
