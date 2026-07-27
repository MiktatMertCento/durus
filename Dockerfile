# syntax=docker/dockerfile:1

FROM node:24-bookworm AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-bookworm AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl gosu \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --create-home --uid 10001 app
WORKDIR /app
COPY --from=backend /out/server /app/server
COPY --from=frontend /app/dist /app/static
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh \
  && chown -R app:app /app
ENV ADDR=:8080 \
    STATIC_DIR=/app/static \
    TZ=Europe/Istanbul
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/app/server"]
