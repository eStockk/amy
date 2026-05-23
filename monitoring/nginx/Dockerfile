FROM nginx:1.27-alpine

RUN apk add --no-cache openssl

COPY nginx.conf.template /etc/nginx/templates/amy.conf.template
COPY start-nginx.sh /usr/local/bin/start-nginx

RUN chmod +x /usr/local/bin/start-nginx

CMD ["start-nginx"]
