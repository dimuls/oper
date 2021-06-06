FROM golang:alpine as builder

WORKDIR /go/src/oper

COPY go.mod go.sum ./
COPY entity entity
COPY service service
COPY operator operator
COPY weber weber

RUN go install ./weber/cmd/weber



FROM alpine:latest

COPY --from=builder /go/bin/weber /usr/sbin/weber

ENTRYPOINT ["/usr/sbin/weber"]
