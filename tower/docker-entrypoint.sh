#!/bin/sh
set -eu

envsubst '${VITE_NEXUS_CORE_URL} ${VITE_NEXUS_SAAS_URL} ${VITE_NEXUS_GRAFANA_URL}' \
  < /etc/nginx/templates/default.conf.template \
  > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
