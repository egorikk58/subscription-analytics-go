# Аналитика подписок на Go

## Запуск

Нужен Docker Desktop или Docker Engine с Compose 2.23.1+. Go, Node.js и базы запускаются в контейнерах. Выполняйте команды из корня репозитория. На Linux используйте `sudo docker compose`, если Docker требует доступ к сокету.

### 1. Заполните `.env`

Для первого запуска скопируйте `.env.example` в `.env` командой `cp .env.example .env`. В PowerShell используйте `Copy-Item .env.example .env`. Если `.env` уже есть, откройте его и поправьте нужные значения. Файл исключён из Git.

| Где открываете стенд | `ANALYTICS_PORT` | `PUBLIC_BASE_URL` |
|---|---|---|
| На своём компьютере | `127.0.0.1:8080` | `http://localhost:8080` |
| Снаружи по IP ВМ, для проверки | `0.0.0.0:8080` | `http://<публичный-IP-ВМ>:8080` |
| Через HTTPS-прокси на хосте | `127.0.0.1:8080` | `https://<ваш-домен>` |

Первое поле задаёт адрес привязки порта Docker, второе — адрес для браузера без слеша в конце. Для проверки по IP разрешите входящий TCP `8080` со своего адреса в группе безопасности ВМ. Вход в панели через интернет используйте по HTTPS. [Деплой и настройки прокси](docs/OPERATIONS.md#деплой)

Замените восемь значений `*_PASSWORD` из шаблона отдельными паролями. Для каждого можно выполнить `openssl rand -hex 24` и вписать результат в `.env`. Пароли базы используются в строках подключения, поэтому подходят буквы, цифры, дефис и подчёркивание.

| Настройка | Назначение |
|---|---|
| `OPS_USERNAME` и `OPS_PASSWORD` | Вход в Grafana, Prometheus и pgAdmin. Логин по умолчанию — `ops` |
| `DB_VIEWER_PASSWORD` | Пароль роли `demo_viewer` для чтения PostgreSQL. pgAdmin подключается с ним автоматически |
| `GRAFANA_ADMIN_PASSWORD` | Внутренняя учётная запись Grafana при первом запуске |
| `POSTGRES_PASSWORD`, `USERS_PASSWORD`, `SUBSCRIPTIONS_PASSWORD`, `PAYMENTS_PASSWORD`, `ANALYTICS_PASSWORD` | Пароли базы для bootstrap и сервисов |

Остальные настройки можно оставить как в шаблоне. `SEED=42` и `USERS_COUNT=auto` создают 31 419 пользователей на чистой БД. Для другого постоянного набора используйте отдельный Compose-проект с собственными volumes.

### 2. Запустите контейнеры

```sh
docker compose --profile observability config --quiet
docker compose --profile observability up --build -d --wait
docker compose ps -a
```

Bootstrap подготавливает БД и завершается с кодом `0`. Analytics опрашивает источники сразу при старте. Появление `snapshot_id` в `/status` означает, что метрики готовы. До первого снимка API метрик отвечает `503` с `Retry-After`.

### 3. Откройте стенд

При локальном запуске используйте ссылки из таблицы. При деплое подставьте свой `PUBLIC_BASE_URL` и добавьте путь.

| Путь | Что проверить | Вход |
|---|---|---|
| [`/`](http://localhost:8080/) | Аналитика, фильтры, когорты и Excel | Свободный |
| [`/status`](http://localhost:8080/status) | `snapshot_id`, следующий опрос и последняя ошибка | Свободный |
| [`/metrics/ltv/summary`](http://localhost:8080/metrics/ltv/summary) | Сводка LTV в JSON | Свободный |
| [`/docs/`](http://localhost:8080/docs/) | Swagger и `Try it out` | Свободный |
| [`/db/`](http://localhost:8080/db/) | Таблицы PostgreSQL и Query Tool | `OPS_USERNAME` / `OPS_PASSWORD` |
| [`/grafana/`](http://localhost:8080/grafana/) | Четыре дашборда | `OPS_USERNAME` / `OPS_PASSWORD` |
| [`/prometheus/targets`](http://localhost:8080/prometheus/targets) | Цели сбора метрик в состоянии `UP` | `OPS_USERNAME` / `OPS_PASSWORD` |

Учётные данные лежат в файле окружения на машине, где запущен стенд. По умолчанию это `.env`. На ВМ выполните `grep -E '^OPS_(USERNAME|PASSWORD)=' .env`. Если pgAdmin отдельно попросит пароль PostgreSQL, используйте `demo_viewer` и `DB_VIEWER_PASSWORD`. [Смена паролей](docs/OPERATIONS.md#доступ)

В меню **Выборка** отсортируйте пользователей по LTV и нажмите ID. В карточке видны даты, тарифы, периоды и суммы оплат, включая возвраты и неуспешные попытки.

## Логи и мониторинг

`slog` пишет JSON в stdout контейнеров. Для HTTP-запросов видны маршрут, код и время ответа. Синхронизация пишет `snapshot published` с номером снимка или `sync failed` с причиной ошибки.

```sh
docker compose logs -f --tail=100 analytics-service
docker compose logs bootstrap
docker compose logs --tail=50 users-service subscriptions-service payments-service
```

Чтобы увидеть трафик в Grafana, выберите **Last 5 minutes** на HTTP-дашборде и несколько раз запросите `/metrics/ltv/summary`. [Нагрузка для проверки графиков](docs/OPERATIONS.md#мониторинг)

Остановка с сохранением данных — `docker compose --profile observability down`. Повторный запуск — `docker compose --profile observability up -d --wait`.

## Что реализовано и какие решения приняты

- Три мока и analytics работают в отдельных Go-контейнерах. Моки доступны во внутренней сети, опубликован только порт analytics.
- Четыре схемы PostgreSQL с разными ролями позволяют создать связанный seed одной транзакцией. Внешние ключи проверяют ID, а `core.Prepare` проверяет связь платежа с подпиской и соответствие суммы цене тарифа в RUB. Analytics читает моки по HTTP. [Архитектура](docs/ARCHITECTURE.md)
- Опрос идёт сразу и затем раз в 10 минут. Источники читаются параллельно с пагинацией, retry/backoff и `updated_since`. PostgreSQL публикует снимок и курсор атомарно. При сбое источника доступен предыдущий снимок, при сбое Redis ответы читаются из PostgreSQL.
- LTV считает оплаты за всю жизнь за вычетом полных возвратов. Для группировки берётся первая подписка. Retention учитывает только завершившиеся периоды. [Формулы и API](docs/METRICS.md)
- Seed воспроизводит размер и историю набора. UUID v4 выбраны ради стабильных ID в моках. Синтетический отток зависит от тарифа, пользователя и стажа. Seed покрывает повторные попытки, возвраты, смену тарифа, отсутствие оплат и незавершённые периоды.
- Бонусы включают RUB/BTC/USDT с курсом исходного платежа для возвратов, UTM-разбивку, Telegram LTV, Prometheus, JSON `slog` и CI.
- Интерфейс на React, TypeScript, Tailwind и shadcn/ui встраивается в Go-бинарник. Доступны выборка пользователей, история покупок, preview по seed и Excel с графиками.

Для production я бы разделил БД источников, ввёл стабильные курсоры и явную границу наблюдений, настроил авторизацию финансовых API и проверяемое восстановление из резервных копий. При масштабировании понадобится координация фонового опроса.

## Проверки, деплой и подробности

`make check` запускает Go race-тесты с отдельными PostgreSQL и Redis, линтер, проверки frontend и валидацию Compose. [Проверки и CI](docs/TESTING.md).

`make deploy` запускает Compose-стек на текущем Docker-хосте. [Настройка сервера и внешнего URL](docs/OPERATIONS.md#деплой)

ИИ использовался для подготовки кода, интерфейса, тестов и документации. Формулы сверены на наборах с вручную рассчитанными результатами и в интеграционных сценариях.
