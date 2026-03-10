FROM node:25-alpine AS builder

ARG VITE_NEXUS_SAAS_URL=http://localhost:8082
ARG VITE_NEXUS_CORE_URL=http://localhost:8080
ARG VITE_NEXUS_API_KEY=
ARG VITE_NEXUS_SCOPES=tools:read,tools:write,policy:read,policy:write,gateway:run,gateway:simulate,admin:console:read,admin:console:write,audit:read,audit:export,mcp:read,mcp:call,a2a:call,admin:secrets
ARG VITE_CLERK_PUBLISHABLE_KEY=
ARG VITE_NEXUS_GRAFANA_URL=http://localhost:3001

ENV VITE_NEXUS_SAAS_URL=$VITE_NEXUS_SAAS_URL \
    VITE_NEXUS_CORE_URL=$VITE_NEXUS_CORE_URL \
    VITE_NEXUS_API_KEY=$VITE_NEXUS_API_KEY \
    VITE_NEXUS_SCOPES=$VITE_NEXUS_SCOPES \
    VITE_CLERK_PUBLISHABLE_KEY=$VITE_CLERK_PUBLISHABLE_KEY \
    VITE_NEXUS_GRAFANA_URL=$VITE_NEXUS_GRAFANA_URL

WORKDIR /workspace
COPY tower/package.json tower/package-lock.json* /workspace/
RUN npm install

COPY tower /workspace
RUN rm -rf node_modules/.vite \
    && printf 'VITE_NEXUS_SCOPES=%s\nVITE_NEXUS_API_KEY=%s\nVITE_NEXUS_CORE_URL=%s\nVITE_NEXUS_SAAS_URL=%s\nVITE_NEXUS_GRAFANA_URL=%s\nVITE_CLERK_PUBLISHABLE_KEY=%s\n' \
       "$VITE_NEXUS_SCOPES" "$VITE_NEXUS_API_KEY" "$VITE_NEXUS_CORE_URL" "$VITE_NEXUS_SAAS_URL" "$VITE_NEXUS_GRAFANA_URL" "$VITE_CLERK_PUBLISHABLE_KEY" \
       | tee .env .env.production .env.local \
    && cat .env \
    && npm run build

FROM nginx:1.27-alpine
ARG VITE_NEXUS_SAAS_URL=http://localhost:8082
ARG VITE_NEXUS_CORE_URL=http://localhost:8080
ARG VITE_NEXUS_GRAFANA_URL=http://localhost:3001
ENV VITE_NEXUS_SAAS_URL=$VITE_NEXUS_SAAS_URL \
    VITE_NEXUS_CORE_URL=$VITE_NEXUS_CORE_URL \
    VITE_NEXUS_GRAFANA_URL=$VITE_NEXUS_GRAFANA_URL

RUN apk add --no-cache gettext \
    && sed -i 's|pid[[:space:]]*/run/nginx.pid;|pid /tmp/nginx.pid;|' /etc/nginx/nginx.conf \
    && chown -R nginx:nginx /usr/share/nginx/html /var/cache/nginx /var/log/nginx /etc/nginx/conf.d /var/run

COPY --from=builder /workspace/dist /usr/share/nginx/html
COPY tower/nginx.conf /etc/nginx/templates/default.conf.template
COPY tower/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

USER nginx
EXPOSE 4173
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://127.0.0.1:4173/ >/dev/null 2>&1 || exit 1
CMD ["/docker-entrypoint.sh"]
