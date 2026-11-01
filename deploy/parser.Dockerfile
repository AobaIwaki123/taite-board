FROM golang:1.27-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
COPY VERSION ./
COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/parser ./cmd/parser

FROM alpine:3.20

RUN apk add --no-cache tzdata ca-certificates

COPY --from=builder /bin/parser /bin/parser

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/bin/parser"]
