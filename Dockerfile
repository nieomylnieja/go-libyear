FROM golang:1.24-alpine3.21 AS builder

WORKDIR /app

COPY ./go.mod ./go.sum ./
COPY ./*.go ./
COPY ./cmd/go-libyear ./cmd/go-libyear
COPY ./internal ./internal

ARG LDFLAGS

RUN CGO_ENABLED=0 go build \
  -ldflags "${LDFLAGS}" \
  -o /artifacts/go-libyear \
  "${PWD}/cmd/go-libyear"

FROM gcr.io/distroless/static-debian12

# https://github.com/opencontainers/image-spec/blob/main/annotations.md
LABEL org.opencontainers.image.authors="nieomylnieja"
LABEL org.opencontainers.image.title="go-libyear"
LABEL org.opencontainers.image.description="Calculate Go module's libyear"
LABEL org.opencontainers.image.source="https://github.com/nieomylnieja/go-libyear"

COPY --from=builder /artifacts/go-libyear ./go-libyear

ENTRYPOINT ["./go-libyear"]
