# Stage 1: Build Modern TypeScript Frontend
FROM node:26-alpine AS builder

ARG APP_VERSION=dev
ENV APP_VERSION=${APP_VERSION}

WORKDIR /app/apps/web
COPY apps/web/package*.json ./
RUN npm ci

COPY apps/web/ ./
RUN npm run build

# Stage 2: Production Nginx Server
FROM nginx:alpine

ARG APP_VERSION=dev
ARG GIT_SHA=""

LABEL org.opencontainers.image.source="https://github.com/AobaIwaki123/taite-board" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.revision="${GIT_SHA}"

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
RUN mkdir -p /var/log/ribonclock && chmod 777 /var/log/ribonclock
COPY --from=builder /app/apps/web/dist/ /usr/share/nginx/html/
COPY data/generated/schedule.json /usr/share/nginx/html/schedule.json
COPY data/generated/schedule.json /usr/share/nginx/html/mock_schedule.json

EXPOSE 8080
