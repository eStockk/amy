#!/bin/sh
set -eu

: "${APP_DOMAIN:=localhost}"
: "${APP_WWW_DOMAIN:=www.localhost}"
: "${APP_IP:=5.83.140.117}"

mkdir -p "/etc/letsencrypt/live/${APP_DOMAIN}" /var/www/certbot /etc/nginx/conf.d

if [ ! -s "/etc/letsencrypt/live/${APP_DOMAIN}/fullchain.pem" ] || [ ! -s "/etc/letsencrypt/live/${APP_DOMAIN}/privkey.pem" ]; then
  openssl req -x509 -nodes -newkey rsa:2048 -days 1 \
    -subj "/CN=${APP_DOMAIN}" \
    -addext "subjectAltName=DNS:${APP_DOMAIN},DNS:${APP_WWW_DOMAIN}" \
    -keyout "/etc/letsencrypt/live/${APP_DOMAIN}/privkey.pem" \
    -out "/etc/letsencrypt/live/${APP_DOMAIN}/fullchain.pem"
fi

envsubst '${APP_DOMAIN} ${APP_WWW_DOMAIN} ${APP_IP}' \
  < /etc/nginx/templates/amy.conf.template \
  > /etc/nginx/conf.d/default.conf

nginx -g 'daemon off;' &
nginx_pid="$!"

while sleep 60; do
  nginx -s reload || true
done &

wait "$nginx_pid"
