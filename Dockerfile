FROM golang:1.5.3

ENTRYPOINT dinger
EXPOSE 8080

COPY . /go/src/github.com/sensiblecodeio/dinger
RUN go install -v github.com/sensiblecodeio/dinger
