FROM golang:alpine as builder

WORKDIR /go/src/oper

COPY go.mod go.sum ./
COPY entity entity
COPY service service
COPY operator operator
COPY pointer pointer

RUN go install ./pointer/cmd/pointer



FROM alpine:latest

COPY --from=builder /go/bin/pointer /usr/sbin/pointer

ENTRYPOINT ["/usr/sbin/pointer"]
