# Сервер GophKeeper

HTTP API. Без `GOPHKEEPER_AUTH_JWT_SECRET` процесс не стартует (`Validate()`).

## Docker Compose

Секрет в `docker-compose.yml` не зашит. Его необходимо передать при запуске сервера:

```bash
GOPHKEEPER_AUTH_JWT_SECRET='secret_key' docker compose up
```

API: `http://localhost:9090`. Adminer: `http://localhost:8081`.