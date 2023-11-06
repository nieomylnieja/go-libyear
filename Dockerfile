FROM golang:1.21-alpine3.18 AS builder

WORKDIR /app

COPY ./go.mod ./go.sum ./
COPY ./*.go ./
COPY ./cmd/ ./cmd
COPY ./internal/ ./internal

ARG LDFLAGS

RUN CGO_ENABLED=0 go build \
  -ldflags "${LDFLAGS}" \
  -o /artifacts/go-libyear \
  "${PWD}/cmd"

FROM scratch

COPY --from=builder /artifacts/go-libyear ./go-libyear

ENTRYPOINT ["./go-libyear"]
