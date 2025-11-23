# Сервис автоматического назначения ревьюверов на Pull Request’ы.

## Быстрый старт
1. Установите Docker.
2. Поднимите контейнеры: `make up` (остановка `make down`).

## Конфигурация

| Переменная         | Значение по умолчанию                                             | Назначение                             |
|--------------------|-------------------------------------------------------------------|----------------------------------------|
| `HTTP_PORT`        | `8080`                                                            | Порт HTTP-сервера                      |
| `DATABASE_URL`     | `postgres://avito:avito@localhost:5432/avito?sslmode=disable`     | DSN PostgreSQL 18                      |
| `LOG_LEVEL`        | `debug`                                                           | `debug`, `info`, `warn`, `error`       |
| `SHUTDOWN_TIMEOUT` | `10s`                                                             | Тайм-аут graceful shutdown             |

## База данных
- PostgreSQL 18 (образ `postgres:18-alpine`).
- Миграции (`internal/migrations/sql/*.sql`) запускаются автоматически при старте сервиса.
- Таблицы: `teams`, `users`, `team_memberships`, `pull_requests`, `pull_request_statuses`, `pr_reviewers`, `schema_migrations`.
- Данные хранятся в volume `pgdata` (каталог `/var/lib/postgresql/data/pgdata` внутри контейнера).

## Допущения и решения
- Пользователь может состоять только в одной команде; повторное добавление меняет привязку.
- Создание команды через `/team/add` идемпотентно обновляет участников (username/isActive).
- Переназначение ищет кандидата в команде заменяемого ревьювера; если активных нет, возвращается `NO_CANDIDATE`.

## Команды Make
| Команда        | Описание                                |
|----------------|-----------------------------------------|
| `make up`      | Поднять Docker-окружение                |
| `make down`    | Оостановить Docker-окружение            |
