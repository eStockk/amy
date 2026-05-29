# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS build

WORKDIR /src

ENV GOPROXY=https://goproxy.cn,direct \
	GOSUMDB=sum.golang.google.cn \
	GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=linux go build -o /out/amy-api ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache su-exec \
	&& adduser -D -H -u 10001 amy

WORKDIR /app
COPY --from=build /out/amy-api /app/amy-api
COPY docker-entrypoint.sh /usr/local/bin/amy-api-entrypoint
RUN chmod +x /usr/local/bin/amy-api-entrypoint

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
 CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/amy-api-entrypoint"]
CMD ["/app/amy-api"]
