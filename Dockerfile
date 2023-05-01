# Source: https://github.com/opsgenie/oec/blob/master/Dockerfile
FROM golang:1.14 as builder
ADD . $GOPATH/src/github.com/opsgenie/oec
WORKDIR $GOPATH/src/github.com/opsgenie/oec/main
RUN export GIT_COMMIT=$(git rev-list -1 HEAD) && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo \
        -ldflags "-X main.OECCommitVersion=$GIT_COMMIT -X main.OECVersion=1.0.1" -o nocgo -o /oec .
FROM --platform=linux/amd64 python:3.7.16-alpine3.17 as base
RUN addgroup -S opsgenie && \
    adduser -S opsgenie -G opsgenie && \
    apk update && \
    apk add --no-cache git ca-certificates gcompat gcc libffi-dev libc-dev libucontext gettext && \
    update-ca-certificates
RUN pip install --upgrade pip==22.3.1 setuptools==65.6.3 && pip install requests==2.27.1 cryptography==40.0.2
COPY --from=builder /oec /opt/oec
RUN mkdir -p /var/log/opsgenie && \
    chown -R opsgenie:opsgenie /var/log/opsgenie && \
    chown -R opsgenie:opsgenie /opt/oec
USER opsgenie
ENTRYPOINT ["/opt/oec"]
