# Multi-stage: build the Vite SPA in the pnpm monorepo, serve the static dist
# with nginx. Build context MUST be the monorepo root (frontend/) so workspace
# packages (@swp/api-client, ui, shared, design-tokens) resolve.
#
#   docker build -f apps/web/Dockerfile --build-arg VITE_API_BASE_URL=/api/v1 .
#
# Memory-capped node heap (NODE_OPTIONS) so the build can't OOM a shared host.
FROM node:22-alpine AS build
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9.6.0 --activate

# Copy the whole workspace (correctness over cache — frozen install needs every
# package.json + the lockfile present).
COPY . .
RUN pnpm install --frozen-lockfile

ARG VITE_API_BASE_URL=/api/v1
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL \
    NODE_OPTIONS=--max-old-space-size=1536
RUN pnpm --filter @swp/web build

FROM nginx:1.27-alpine AS run
COPY apps/web/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/apps/web/dist /usr/share/nginx/html
EXPOSE 80
