# build image
FROM golang:1.13.1-alpine3.10 as builder

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh

RUN go get -v github.com/nats-io/nats.go/ && \
    go get -v github.com/valyala/fasthttp && \
    go get -v golang.org/x/net/proxy

COPY . /app/
WORKDIR /app

# Test then build app
RUN CGO_ENABLED=0 go test -v
RUN go build -v crawler.go


# runtime image
FROM alpine:latest
COPY --from=builder /app/crawler /app/

WORKDIR /app/
CMD ["./crawler"]