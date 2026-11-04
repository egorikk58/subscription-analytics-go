# Аналитика подписок на Go

## Запуск за несколько команд

Нужен запущенный Docker Desktop или Docker Engine с Compose v2. Go, Node.js и базы на хосте не нужны. Команды выполняются из корня репозитория.

```sh
git clone https://github.com/egorikk58/subscription-analytics-go.git
cd subscription-analytics-go
cp .env.example .env
docker compose --profile observability up --build -d --wait
```

В PowerShell вместо `cp`: `Copy-Item .env.example .env`. Для первого локального запуска подходят значения шаблона. Перед внешним деплоем замените пароли в `.env` и задайте `PUBLIC_BASE_URL`. Сам `.env` исключён из Git.

Откройте **[localhost:8080](http://localhost:8080)**. При первом запуске bootstrap создаёт данные, затем analytics загружает первый снимок. До его готовности API метрик возвращает `503` с `Retry-After`; состояние видно в `/status`.

| Куда зайти | Что проверить |
|---|---|
| [Приложение](http://localhost:8080) | LTV, продления, когорты, фильтры и выборку пользователей |
| [Swagger](http://localhost:8080/docs/) | Контракт API и `Try it out` |
| [pgAdmin](http://localhost:8080/db/) | Четыре схемы PostgreSQL и SQL-запросы; подключение уже настроено |
| [Grafana](http://localhost:8080/grafana/) | Четыре готовых дашборда сервиса и хранилищ |
| [Prometheus](http://localhost:8080/prometheus/) | Пять целей сбора, PromQL и alert rules |

Для pgAdmin, Grafana и Prometheus: логин `ops`, пароль — значение `OPS_PASSWORD` из `.env`. pgAdmin подключается от роли `demo_viewer` с правом чтения.

## Быстрая проверка

```sh
curl http://localhost:8080/status
curl http://localhost:8080/metrics/ltv/summary
curl 'http://localhost:8080/metrics/retention?plan_id=monthly&max_periods=12'
docker compose --profile demo run --build --rm demo-traffic
```

Последняя команда на три минуты создаёт до 10 GET/сек. В Grafana выберите последние 5 минут и обновление 15 секунд; графики изменятся через 30–60 секунд. [Подробнее о нагрузке и SQL-примерах](docs/DEMO.md).

В приложении можно поменять тариф, даты первой подписки и UTM; открыть пользователя по Telegram ID; создать preview с другим seed и вернуться к рабочему снимку. Preview не меняет данные PostgreSQL.

Остановка с сохранением данных: `docker compose --profile observability down`. Повторный запуск: `docker compose --profile observability up -d --wait`. Для нового постоянного набора нужна отдельная БД: смена seed не перезаписывает существующие данные.

## Что реализовано и какие решения приняты

- Четыре HTTP-сервиса на Go: пользователи, подписки/тарифы, платежи и analytics. Источники изолированы; снаружи открыт только analytics.
- PostgreSQL разделён на четыре схемы с отдельными ролями. Общая транзакция генератора и внешние ключи обеспечивают согласованность связей между моками; analytics читает их только по HTTP. Redis ускоряет выдачу, при его отказе остаётся PostgreSQL-кеш.
- Опрос стартует сразу, затем раз в 10 минут: параллельные источники, пагинация, retry/backoff и `updated_since`. Снимок и курсор публикуются одной транзакцией; неудачный опрос сохраняет предыдущие данные.
- LTV учитывает lifetime-оплаты и полные возвраты. Группа тарифа определяется первой подпиской. Retention исключает незавершённые периоды; правила зрелости и календарных границ описаны в [метриках](docs/METRICS.md).
- Seed воспроизводит данные и UUID v4: для моков важнее повторяемость, чем сортировка ID по времени. Новый набор по умолчанию содержит 25–100 тысяч пользователей и 24 месяца истории; есть retries, refunds, смена тарифов, нулевые оплаты и незавершённые периоды.
- Бонусы: RUB/BTC/USDT с точной конвертацией, UTM-разбивка, Telegram LTV, Prometheus и JSON `slog`. Котировки моков синтетические; возврат использует курс исходной оплаты. [Детали](docs/BONUSES.md).
- Фронтенд: React, TypeScript, Tailwind, shadcn/ui и Recharts; сборка встраивается в Go-бинарник. [Устройство UI](docs/FRONTEND.md).

Для production потребуются независимые БД, потоковые обновления со стабильными курсорами, авторизация финансовых API, backup/restore и sync leader при масштабировании. Ограничения текущей версии и порядок улучшений — в [backlog](docs/BACKLOG.md).

## Проверки, деплой и подробности

Тесты, линтер и сценарии CI: [docs/TESTING.md](docs/TESTING.md). Деплой через Docker и настройка внешнего URL: [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md).

[Архитектура, кеш и синхронизация](docs/ARCHITECTURE.md) · [Метрики и API](docs/METRICS.md) · [Мониторинг и доступ](docs/OPERATIONS.md) · [Что изменить для production](docs/BACKLOG.md).

ИИ использовался для подготовки кода, интерфейса, тестов и документации. Принятые формулы и ограничения проверяются отдельными ручными fixtures и интеграционными сценариями; автор решения отвечает за объяснение и дальнейшие изменения кода.
