# build image
FROM golang:1.12.7-alpine3.10 as builder

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh

RUN go get -v github.com/nats-io/nats.go/ && \
    go get -v github.com/valyala/fasthttp && \
    go get -v golang.org/x/net/proxy

COPY . /app/
WORKDIR /app

RUN go build -v crawler.go
RUN go build -v feeder.go


# runtime image
FROM alpine:latest
COPY --from=builder /app/crawler /app/
COPY --from=builder /app/feeder /app/

WORKDIR /app/
CMD ["./crawler"]