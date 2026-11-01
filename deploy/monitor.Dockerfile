FROM golang:1.27-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
COPY VERSION ./
COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/monitor ./cmd/monitor

FROM alpine:3.20

RUN apk add --no-cache tzdata ca-certificates

COPY --from=builder /bin/monitor /bin/monitor

USER 65532:65532

EXPOSE 8082 9090

ENTRYPOINT ["/bin/monitor"]
