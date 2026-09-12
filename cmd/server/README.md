# Сервер GophKeeper

HTTP API. Без `GOPHKEEPER_AUTH_JWT_SECRET` процесс не стартует (`Validate()`). Нужны также S3 (`s3.endpoint`, bucket, ключи) — в compose это MinIO.

## Docker Compose

Секреты в `docker-compose.yml` не зашиты. Их необходимо передать при запуске:

```bash
GOPHKEEPER_AUTH_JWT_SECRET='secret_key' \
MINIO_ROOT_PASSWORD='minio_password' \
GOPHKEEPER_S3_SECRET_KEY='minio_password' \
docker compose up -d --build
```

`MINIO_ROOT_PASSWORD` и `GOPHKEEPER_S3_SECRET_KEY` должны совпадать.

API: `http://localhost:9090`. gRPC файлы: `localhost:9091`. Adminer: `http://localhost:8081`. MinIO S3: `http://localhost:9000` (консоль `:9001`).
