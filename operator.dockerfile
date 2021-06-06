FROM golang:alpine as builder

WORKDIR /go/src/oper

COPY go.mod go.sum ./
COPY entity entity
COPY service service
COPY operator operator

RUN go install ./operator/cmd/operator



FROM alpine:latest

COPY --from=builder /go/bin/operator /usr/sbin/operator

ENTRYPOINT ["/usr/sbin/operator"]
