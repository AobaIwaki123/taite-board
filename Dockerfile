# Stage 1: Build Modern TypeScript Frontend
FROM node:22-alpine AS builder

WORKDIR /app/apps/web
COPY apps/web/package*.json ./
RUN npm ci

COPY apps/web/ ./
RUN npm run build

# Stage 2: Production Nginx Server
FROM nginx:alpine

LABEL org.opencontainers.image.source="https://github.com/AobaIwaki123/taite-board"

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/apps/web/dist/ /usr/share/nginx/html/
COPY data/generated/schedule.json /usr/share/nginx/html/schedule.json
COPY data/generated/schedule.json /usr/share/nginx/html/mock_schedule.json

EXPOSE 8080
