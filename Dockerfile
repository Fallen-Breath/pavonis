FROM golang:1.24 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 go build -o pavonis cmd/pavonis/pavonis.go

FROM alpine:latest
RUN apk add --no-cache tzdata  # so TZ environment works

WORKDIR /root
COPY --from=builder /build/pavonis /usr/bin/pavonis

ENTRYPOINT ["/usr/bin/pavonis"]
CMD ["-c", "/etc/pavonis/config.yml"]
