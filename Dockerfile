FROM golang:1.11.1 AS builder

WORKDIR /kubewatch

ADD . .

RUN CGO_ENABLED=0 go build -mod=vendor -a -installsuffix cgo -o /tmp/kubewatch

######

FROM alpine:3.7

LABEL maintainer="xuyuanp@gmail.com"

RUN apk add --no-cache -U tzdata ca-certificates

COPY --from=builder /tmp/kubewatch /bin/kubewatch

ENTRYPOINT ["/bin/kubewatch"]
