FROM node:20-alpine AS builder

ARG VITE_NEXUS_CORE_URL=http://localhost:8080
ARG VITE_NEXUS_AI_OPERATORS_URL=http://localhost:8000
ARG VITE_NEXUS_API_KEY=
ARG VITE_NEXUS_SCOPES=admin:console:read,admin:console:write,audit:read
ARG VITE_AI_OPERATORS_ACTION_KEY=operator-internal-key

ENV VITE_NEXUS_CORE_URL=$VITE_NEXUS_CORE_URL \
    VITE_NEXUS_AI_OPERATORS_URL=$VITE_NEXUS_AI_OPERATORS_URL \
    VITE_NEXUS_API_KEY=$VITE_NEXUS_API_KEY \
    VITE_NEXUS_SCOPES=$VITE_NEXUS_SCOPES \
    VITE_AI_OPERATORS_ACTION_KEY=$VITE_AI_OPERATORS_ACTION_KEY

WORKDIR /workspace
COPY nexus-tower/package.json nexus-tower/package-lock.json* /workspace/
RUN npm install

COPY nexus-tower /workspace
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=builder /workspace/dist /usr/share/nginx/html
COPY nexus-tower/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 4173
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://127.0.0.1:4173/ >/dev/null 2>&1 || exit 1
CMD ["nginx", "-g", "daemon off;"]
