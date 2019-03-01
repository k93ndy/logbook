FROM golang:1.12.0-alpine3.9 as builder
COPY ./* /go/src/logbook/
WORKDIR /go/src/logbook/
RUN set -ex \
    && apk add --no-cache curl git \
    && curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh \ 
    && dep ensure \
    && CGO_ENABLED=1 go build -o logbook main.go

FROM scratch
COPY --from=builder /go/src/logbook/logbook* /
ENTRYPOINT ["/logbook"]
