FROM golang:1.24 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 go build -o pavonis cmd/pavonis/pavonis.go

FROM alpine:latest
RUN apk add --no-cache tzdata  # so TZ environment works
WORKDIR /app
COPY --from=builder /build/pavonis /app/pavonis

ENTRYPOINT ["/app/pavonis"]
CMD ["/app/pavonis", "-c", "config.yml"]
