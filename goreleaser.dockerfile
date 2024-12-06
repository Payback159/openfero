# build stage
FROM alpine@sha256:21dc6063fd678b478f57c0e13f47560d0ea4eeba26dfc947b2a4f81f686b9f45 AS build-env

RUN echo "openfero:x:10001:10001:OpenFero user:/app:/sbin/nologin" >> /etc/passwd_single && \
    echo "openfero:x:10001:" >> /etc/group_single

# final stage
FROM scratch

WORKDIR /app

COPY openfero /app/
COPY web/ /app/web/
COPY --from=build-env /etc/passwd_single /etc/passwd
COPY --from=build-env /etc/group_single /etc/group
USER 10001

CMD ["/app/openfero"]
