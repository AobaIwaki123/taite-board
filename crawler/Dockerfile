FROM mcr.microsoft.com/playwright:v1.50.0-noble

WORKDIR /app

COPY package.json ./
RUN npm install --omit=dev

COPY index.mjs ./

USER pwuser
ENTRYPOINT ["node", "index.mjs"]
