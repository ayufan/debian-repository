ARG ARCH
FROM ${ARCH}golang:alpine as build

COPY . $GOPATH/src/github.com/ayufan/debian-repository
RUN cd $GOPATH/src/github.com/ayufan/debian-repository && \
  go install -v ./...

ARG ARCH
FROM ${ARCH}alpine as release
COPY --from=build /go/bin/debian-repository /

VOLUME ["/cache"]

ENV REPOSITORY_CACHE=/cache

ENTRYPOINT ["/debian-repository"]
