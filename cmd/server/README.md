# Сервер GophKeeper

HTTP API. Без `GOPHKEEPER_AUTH_JWT_SECRET` процесс не стартует (`Validate()`). Нужны также S3 (`s3.endpoint`, bucket, ключи) — в compose это MinIO.

## Docker Compose

Секрет в `docker-compose.yml` не зашит. Его необходимо передать при запуске сервера:

```bash
GOPHKEEPER_AUTH_JWT_SECRET='secret_key' docker compose up -d --build
```

API: `http://localhost:9090`. gRPC файлы: `localhost:9091`. Adminer: `http://localhost:8081`. MinIO S3: `http://localhost:9000` (консоль `:9001`).
