FROM node:22-bookworm-slim

WORKDIR /app

COPY package.json ./
RUN npm install --no-audit --no-fund

COPY tsconfig.json vitest.config.ts wrangler.jsonc ./
COPY src ./src
COPY scripts ./scripts
COPY metadata.json ./metadata.json
COPY images ./images

ENV WRANGLER_SEND_METRICS=false
ENV PERSIST_TO=/data
EXPOSE 8787

CMD ["npx", "wrangler", "dev", "--ip", "0.0.0.0", "--port", "8787", "--local", "--persist-to", "/data"]
