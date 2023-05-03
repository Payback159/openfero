# build stage
# golang:1.20.3-bullseye
FROM golang@sha256:05a961fb54f63ba541367e6aefc9a7d4bd9ea2a9f019708c4cb4960e2cbf58fb AS build-env

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
