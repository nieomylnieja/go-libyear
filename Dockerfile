FROM golang:1.21-alpine3.18 AS builder

WORKDIR /app

COPY ./ ./

ARG LDFLAGS

RUN CGO_ENABLED=0 go build \
  -ldflags "${LDFLAGS}" \
  -o /artifacts/golibyear \
  "${PWD}/cmd"

FROM scratch

COPY --from=builder /artifacts/golibyear ./golibyear

ENTRYPOINT ["./golibyear"]
