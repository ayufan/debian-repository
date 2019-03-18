FROM alpine

COPY . /go/src/github.com/ayufan/debian-repository

ENV GOROOT=/usr/lib/go \
    GOPATH=/go \
    GOBIN=/go/bin \
    PATH=$PATH:$GOROOT/bin:$GOPATH/bin

RUN apk add -U git ca-certificates go build-base && \
  go get -v github.com/ayufan/debian-repository && \
  apk del git go build-base && \
  rm -rf /go/src /go/pkg /var/cache/apk/

VOLUME ["/cache"]

ENV REPOSITORY_CACHE=/cache

ENTRYPOINT ["/go/bin/debian-repository"]
