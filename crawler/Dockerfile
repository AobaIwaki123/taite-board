FROM mcr.microsoft.com/playwright:v1.50.0-noble

ARG APP_VERSION=dev
ARG GIT_SHA=""

LABEL org.opencontainers.image.source="https://github.com/AobaIwaki123/taite-board" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.revision="${GIT_SHA}"

ENV APP_VERSION=${APP_VERSION}

WORKDIR /app

COPY package.json ./
RUN npm install --omit=dev

COPY index.mjs ./

USER pwuser
ENTRYPOINT ["node", "index.mjs"]
