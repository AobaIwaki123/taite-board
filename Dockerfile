FROM nginx:alpine

LABEL org.opencontainers.image.source="https://github.com/AobaIwaki123/taite-board"

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY web/ /usr/share/nginx/html/
COPY docs/mock_schedule.json /usr/share/nginx/html/mock_schedule.json

EXPOSE 8080

