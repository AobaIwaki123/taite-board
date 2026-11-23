FROM python:3.9-alpine

ARG APP_VERSION=dev
ARG GIT_SHA=""

LABEL org.opencontainers.image.source="https://github.com/AobaIwaki123/taite-board" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.revision="${GIT_SHA}"

ENV APP_VERSION=${APP_VERSION} \
    PYTHONDONTWRITEBYTECODE=1

RUN apk add --no-cache tzdata

WORKDIR /app
COPY scripts/report_access_counts.py /app/report_access_counts.py

USER 65532:65532

ENTRYPOINT ["python3", "/app/report_access_counts.py"]
