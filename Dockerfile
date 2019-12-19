# build image
FROM golang:1.13.5-alpine3.10 as builder

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh

COPY . /app/
WORKDIR /app

# Test then build app
RUN CGO_ENABLED=0 go test -v
RUN go build -v


# runtime image
FROM alpine:latest
COPY --from=builder /app/crawler /app/

WORKDIR /app/
CMD ["./crawler"]
