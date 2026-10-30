FROM nginx:alpine

LABEL org.opencontainers.image.source="https://github.com/AobaIwaki123/taite-board"

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY apps/web/ /usr/share/nginx/html/
COPY data/generated/schedule.json /usr/share/nginx/html/schedule.json
COPY data/generated/schedule.json /usr/share/nginx/html/mock_schedule.json

EXPOSE 8080
