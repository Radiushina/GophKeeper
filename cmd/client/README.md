# Клиент GophKeeper

CLI к удалённому API. Учётку создаёт сервер (`POST /api/v1/user/register`). Клиент только логинится. После `login` JWT хранится в памяти этого процесса и сам уходит в `Authorization` на следующих запросах. `exit` или новый запуск — сессия сбрасывается.

Перед запуском клиента сервер должен быть уже запущен.

## Запуск

Из корня репозитория:

```bash
go run ./cmd/client --server http://localhost:9090
```

Терминальный UI (Charm), REPL при этом остаётся:

```bash
go run ./cmd/client --server http://localhost:9090 -tui
```

Из REPL та же оболочка: команда `tui`. `q` возвращает в `>`.

Сборка под Win/Linux/macOS: `make build-client`.

## Команды в сессии

После старта появится `>`.
Аутентификация:
```text
login -login alice -password secret
```

Терминальный UI (из REPL, после `q` снова `>`):
```text
tui
```

Заметки (после `login` в этой же сессии; текст шифруется vault key до отправки):
```text
note-add -text "secret" -meta "work"
note-list
note-get -id 550e8400-e29b-41d4-a716-446655440000
note-update -id 550e8400-e29b-41d4-a716-446655440000 -text "changed"
note-delete -id 550e8400-e29b-41d4-a716-446655440000
```

Версия и дата сборки бинарника:

```text
version
```
Выход:
```text
exit
```

`go run` берёт дату и коммит из git-штампа компилятора. `make build-client` дополнительно проставляет версию/дату через `-ldflags`.

`quit` — то же, что `exit`.
