FROM golang:1.21-alpine3.18 AS builder

WORKDIR /src

COPY ./test/test_server.go ./
RUN go mod init test_server

RUN go build -o /artifacts/test_server .

FROM bats/bats:v1.10.0

RUN apk --no-cache --update add gettext git

COPY ./test ./test
COPY --from=go-libyear-test-bin ./go-libyear /bin/go-libyear
COPY --from=builder /artifacts/test_server /bin/test_server

# Required for bats pretty printing.
ENV TERM=linux
