FROM golang:1.27-alpine AS builder

ARG APP_VERSION=dev

WORKDIR /app
COPY go.mod go.sum* ./
COPY VERSION ./
COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X github.com/AobaIwaki123/taite-board/internal/version.BuildVersion=${APP_VERSION}" -o /bin/parser ./cmd/parser

FROM alpine:3.20

ARG APP_VERSION=dev
ARG GIT_SHA=""

LABEL org.opencontainers.image.source="https://github.com/AobaIwaki123/taite-board" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.revision="${GIT_SHA}"

RUN apk add --no-cache tzdata ca-certificates

COPY --from=builder /bin/parser /bin/parser

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/bin/parser"]
