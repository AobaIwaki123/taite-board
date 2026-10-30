FROM python:3.9-alpine

RUN apk add --no-cache tzdata

WORKDIR /app
COPY scripts/report_access_counts.py /app/report_access_counts.py

ENV PYTHONDONTWRITEBYTECODE=1
USER 65532:65532

ENTRYPOINT ["python3", "/app/report_access_counts.py"]
