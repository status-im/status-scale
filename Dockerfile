FROM golang:1.10-alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git
RUN go get github.com/tylertreat/comcast

FROM statusteam/status-go:latest

RUN apk add --no-cache ca-certificates bash ipfw iptables ip6tables iproute2 sudo
COPY --from=builder /go/bin/comcast /usr/local/bin/
