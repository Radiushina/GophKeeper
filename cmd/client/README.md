# Клиент GophKeeper

CLI к удалённому API. После `register`/`login` JWT хранится в памяти этого процесса и сам уходит в `Authorization` на следующих запросах. `exit` или новый запуск — сессия сбрасывается.

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
Регистарция:
```text
register -login alice -password secret
```
Аутентификация:
```text
login -login alice -password secret
```

Терминальный UI (из REPL, после `q` снова `>`):
```text
tui
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
