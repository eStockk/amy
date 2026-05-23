#!/bin/sh
set -eu

: "${APP_DOMAIN:=localhost}"
: "${APP_WWW_DOMAIN:=www.localhost}"
: "${LETSENCRYPT_EMAIL:=admin@example.com}"

if [ "$APP_DOMAIN" != "localhost" ] && [ "$APP_DOMAIN" != "" ]; then
  if openssl x509 -in "/etc/letsencrypt/live/${APP_DOMAIN}/fullchain.pem" -issuer -noout 2>/dev/null | grep -qi "Amy self-signed\\|CN = ${APP_DOMAIN}"; then
    rm -rf "/etc/letsencrypt/live/${APP_DOMAIN}" "/etc/letsencrypt/archive/${APP_DOMAIN}" "/etc/letsencrypt/renewal/${APP_DOMAIN}.conf"
  fi

  certbot certonly \
    --webroot \
    --webroot-path /var/www/certbot \
    --email "$LETSENCRYPT_EMAIL" \
    --agree-tos \
    --non-interactive \
    --keep-until-expiring \
    -d "$APP_DOMAIN" \
    -d "$APP_WWW_DOMAIN" || true
fi

trap 'exit 0' TERM INT
while :; do
  certbot renew --webroot --webroot-path /var/www/certbot --quiet || true
  sleep 12h &
  wait "$!"
done
