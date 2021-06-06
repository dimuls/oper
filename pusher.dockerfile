FROM golang:alpine as builder

WORKDIR /go/src/oper

COPY go.mod go.sum ./
COPY entity entity
COPY service service
COPY operator operator
COPY pusher pusher

RUN go install ./pusher/cmd/pusher



FROM alpine:latest

COPY --from=builder /go/bin/pusher /usr/sbin/pusher

ENTRYPOINT ["/usr/sbin/pusher"]