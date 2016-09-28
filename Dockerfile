FROM golang:1.7-alpine

ENTRYPOINT dinger
EXPOSE 8080

COPY . /go/src/github.com/sensiblecodeio/dinger
RUN go install -v github.com/sensiblecodeio/dinger
