FROM golang:1.4

ENTRYPOINT dinger
EXPOSE 8080 

COPY . /go/src/github.com/scraperwiki/dinger
RUN go get -v github.com/scraperwiki/dinger
