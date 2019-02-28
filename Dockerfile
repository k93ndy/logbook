FROM golang:1.12.0-alpine3.9
COPY ./* /go/src/logbook/
WORKDIR /go/src/logbook/
RUN set -ex \
    && apk add --no-cache curl git \
    && curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh \ 
    && echo $GOPATH \
    && dep ensure \
    && CGO_ENABLED=1 go build -o logbook main.go
ENTRYPOINT ["/go/src/logbook/logbook"]
