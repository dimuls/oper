FROM golang:alpine as builder

WORKDIR /go/src/oper

COPY go.mod go.sum ./
COPY entity entity
COPY service service
COPY operator operator
COPY accounter accounter

RUN go install ./accounter/cmd/accounter



FROM alpine:latest

COPY --from=builder /go/bin/accounter /usr/sbin/accounter

ENTRYPOINT ["/usr/sbin/accounter"]
