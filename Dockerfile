FROM golang:1.12.0-alpine3.9 as builder
COPY ./ /go/src/github.com/k93ndy/logbook/
WORKDIR /go/src/github.com/k93ndy/logbook/
RUN set -ex \
    && apk add --no-cache curl git \
    && curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh \ 
    && dep ensure \
    && ls -lar \
    && cat config/config.go \
    && CGO_ENABLED=0 go build -o logbook main.go

FROM scratch
COPY --from=builder /go/src/github.com/k93ndy/logbook/logbook /logbook
ENTRYPOINT ["/logbook"]
