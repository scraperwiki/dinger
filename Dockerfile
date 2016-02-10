FROM golang:1.5.3

ENTRYPOINT dinger
EXPOSE 8080

COPY . /go/src/github.com/scraperwiki/dinger
RUN go get -v github.com/scraperwiki/dinger
