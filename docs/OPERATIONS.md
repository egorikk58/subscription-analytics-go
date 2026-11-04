# Swagger, база данных и мониторинг

## Запуск полного окружения

```sh
cp .env.example .env
# Заменить примеры паролей и задать PUBLIC_BASE_URL без завершающего слеша.
docker compose --profile observability up --build -d --wait
```

Либо `make up-observability`. Нужен запущенный Docker Engine. Базовый запуск без профиля включает API и Swagger; профиль `observability` добавляет pgAdmin, Prometheus и Grafana. Из наружных портов опубликован только analytics. Внутренний listener `9091` для сбора метрик не имеет host port.

| Адрес относительно PUBLIC_BASE_URL | Назначение |
|---|---|
| `/docs/` | Self-hosted Swagger UI, Try it out и выбор контракта |
| `/openapi.json` | OpenAPI 3.0.3 для analytics, dashboard и preview |
| `/openapi-mocks.json` | Контракты внутренних users/subscriptions/payments; браузер не имеет доступа к их Docker DNS |
| `/db/` | pgAdmin 4 с готовым подключением PostgreSQL |
| `/grafana/` или `/grafana/d/subscope-overview` | Стартовый обзор: здоровье 4 сервисов, снимок, хранилища, HTTP, кеш и native валюты |
| `/grafana/d/subscope-http` | HTTP, latency и Go runtime |
| `/grafana/d/subscope-sync` | Синхронизация, источники, кеш и возраст снимка |
| `/grafana/d/subscope-storage` | PostgreSQL и Redis |
| `/prometheus/` | PromQL, targets и alert rules |
| `/metrics` | Prometheus exposition через защищённый публичный маршрут |

Кнопки инструментов находятся также в разделе «Источники» React-приложения.

Все четыре дашборда находятся в папке Subscope и связаны кнопками навигации. Общий обзор содержит 14 панелей и открывается по умолчанию через `GF_USERS_HOME_PAGE=/d/subscope-overview`. У существующей учётной записи оператора также выбран homeDashboardUID=subscope-overview. Персональная настройка хранится в Grafana volume; новый экземпляр использует Compose default. Маршрут welcome с «Create your first dashboard» больше не является стартовым. Настройки home/preferences: [официальный Preferences API](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/api-legacy/preferences/).

## Доступ

`/db/`, `/grafana/`, `/prometheus/` и публичный `/metrics` требуют HTTP Basic: `OPS_USERNAME` / `OPS_PASSWORD`. Пустой OPS_PASSWORD отключает доступ (503), неверные credentials дают 401. Заголовок Authorization не передаётся upstream. Grafana получает подтверждённое gateway имя в X-WEBAUTH-USER; входящий клиентский заголовок заменяется. Grafana находится в отдельной внутренней сети observability, anonymous и signup отключены. Авторизованный оператор получает роль Editor для Explore/запросов; provisioned dashboards сохраняются в исходниках.

Перед внешним использованием нужен HTTPS на входе; PUBLIC_BASE_URL должен содержать фактические scheme/host/port, например `https://analytics.example.test`. При изменении адреса обновить переменную и пересоздать сервисы. В браузере пароли не вшиты в frontend и не передаются query string.

В pgAdmin после gateway входа уже импортировано подключение «Аналитика подписок — PostgreSQL» в группе «Демо»: postgres:5432, база POSTGRES_DB, роль demo_viewer. Дополнительный web-login не нужен: single-user desktop mode расположен за Basic-auth gateway. Порт pgAdmin не опубликован. Пароль подключения берётся из DB_VIEWER_PASSWORD через runtime pgpass, копируется entrypoint с mode 0600 и не вшивается в image или Git. Исходный шаблон servers.json не содержит пароля. Профиль, настройки и сохранённые SQL-запросы хранятся в отдельном pgadmin-data volume.

Роль demo_viewer позволяет просматривать схемы/таблицы и выполнять SELECT в Query Tool. Права записи не расширяются. Adminer заменён pgAdmin. Техническая схема субдиректории соответствует [официальной документации pgAdmin](https://www.pgadmin.org/docs/pgadmin4/9.19/container_deployment.html): gateway задаёт X-Script-Name=/db, сохраняет внешний Host и протокол.

Крупные JS/CSS assets pgAdmin сжимаются gateway в gzip, если браузер принимает это кодирование. Сжатие не применяется к HTML и SQL-результатам. Для `/db/` выделен deadline 2 минуты, чтобы загрузка через медленный HTTPS-туннель не обрывалась; API метрик сохраняет deadline 15 секунд. Первое открытие pgAdmin через туннель может занимать несколько десятков секунд.

Только `/db/static/` получает private browser cache на час: редакторы pgAdmin открываются в iframe и повторно используют большие bundles. Это предотвращает их повторную загрузку для каждого Query Tool. Динамические страницы и SQL-результаты остаются `no-store`. Сетевое обращение к assets также проходит gateway authentication.

Bootstrap создаёт optional роль demo_viewer, если задан DB_VIEWER_PASSWORD, и выдаёт SELECT на четыре схемы. У роли default_transaction_read_only=on и statement_timeout=10s; прав записи нет. Пользоваться postgres/superuser для демонстрации не требуется. Создание роли происходит и при повторном bootstrap с уже заполненной БД, до проверки существующего seed; данные не пересоздаются. Изменение env не ротирует пароль уже созданной роли. Аналогично GRAFANA_ADMIN_PASSWORD и persistent Grafana volume: процедуры смены credentials выполнять явно.

GRAFANA_ADMIN_PASSWORD задаёт внутреннюю админскую учётную запись при первом старте. Обычно оператор заходит через gateway auth-proxy; login form отключена. БД и Redis не получают внешних портов. Swagger использует локальные assets Docker-образа, онлайн validator отключён.

## Метрики

Основой являются RED для HTTP (rate/errors/duration), health/saturation для хранилищ и стандартные collectors Go/process. Бизнес-платежи и деньги через float64 не пересчитываются; Prometheus хранит технические показатели и размеры набора.

Бонусы добавляют `subscope_dataset_payment_records{currency}` для RUB/BTC/USDT и график Native payment records в Sync/Cache. Это число записей, включая failed/refund. UTM и Telegram ID не попадают в labels; правила FX и новые API описаны в [BONUSES.md](BONUSES.md).

| Семейство | Тип / смысл |
|---|---|
| `subscope_http_requests_total` | Counter: method, нормализованный route, status |
| `subscope_http_request_duration_seconds` | Histogram: p95/p99 через histogram_quantile |
| `subscope_http_response_size_bytes` | Histogram размера ответа |
| `subscope_http_requests_in_flight` | Gauge активных запросов |
| `go_*`, `process_*` | Стандартные goroutines, heap, GC, CPU, RSS и uptime |
| `subscope_sync_total`, `subscope_sync_duration_seconds` | Counter/Histogram с result=success/error |
| `subscope_sync_in_progress`, `subscope_sync_interval_seconds` | Текущее выполнение и настроенный интервал |
| `subscope_sync_last_success_timestamp_seconds` | Последний успешный commit; восстанавливается из PG |
| `subscope_snapshot_available`, `subscope_snapshot_data_timestamp_seconds` | Наличие и время опубликованного снимка |
| `subscope_dataset_users/subscriptions/payments` | Фактические размеры опубликованного набора, в том числе после restart |
| `subscope_source_requests_total`, `subscope_source_request_duration_seconds` | Все HTTP-попытки, включая retry; latency до получения headers. Обрывы body видны через итог ошибки sync |
| `subscope_source_fetch_total`, `subscope_source_fetch_duration_seconds` | Полная загрузка источника со всеми страницами/повторами и результатом success/error/canceled |
| `subscope_cache_lookups_total` | layer=redis/postgres, result=hit/miss/error; одна запись на singleflight operation |
| `subscope_response_build_duration_seconds` | Подготовка и сохранение фильтрованного ответа при cache miss |
| `subscope_postgres_up`, `subscope_postgres_pool_connections` | Probe и total/idle/acquired/max pgx pool |
| `subscope_postgres_database_size_bytes`, `subscope_postgres_connections` | Размер базы и соединения |
| `subscope_postgres_transactions_total`, `subscope_postgres_deadlocks_total`, `subscope_postgres_temp_bytes_total` | Счётчики commit/rollback, deadlocks и временных записей |
| `subscope_redis_up`, `subscope_redis_memory_used_bytes`, `subscope_redis_memory_limit_bytes`, `subscope_redis_connected_clients` | Probe, память и клиенты; maxmemory=0 означает отсутствие лимита |
| `subscope_redis_evicted_keys_total`, `subscope_redis_keyspace_hits_total`, `subscope_redis_keyspace_misses_total` | Накопленные Redis counters |
| `subscope_dependency_last_check_timestamp_seconds` | Время последней проверки зависимостей |

Сбор и опрос зависимостей — каждые15s. Scrape handler отдаёт gauges/counters без SQL/Redis-запросов; ограниченный контекст фонового poll защищает от зависания. Размеры dataset восстанавливаются при смене версии снимка, не при каждом HTTP-запросе. Счётчики процесса начинаются заново после restart; timestamp успешного снимка сохраняется. Для throughput counters используется rate/increase, а не сравнение абсолютных значений.

Метки ограничены service, method, route, status, source, layer/result и state. В них нет user_id, Telegram ID, query string, seed, произвольных URL, сообщений ошибок или паролей. Неизвестные пути объединяются в unmatched. В HTTP RPS/5xx dashboard исключены служебные health/assets/tool routes; latency histogram включает все HTTP routes. Source p95 специально подписан «to headers».

Графики общих PostgreSQL stats используют только series analytics-service: все четыре сервиса подключены к одной базе, суммирование этих копий увеличило бы числа вчетверо. pgx pool показан отдельно для каждого сервиса. Нулевое число событий и отсутствие выборки не превращаются в выдуманные проценты.

## Provisioning и правила

В Git входят JSON dashboards, YAML datasource/provider, prometheus.yml и alerts.yml. Бинарные TSDB/SQLite данные, volumes, пароли, screenshots и exports остаются вне Git.

Alert rules: недоступный scrape target, PostgreSQL/Redis failure, отсутствие первого снимка более5min, stale snapshot после двух sync intervals +120s и HTTP5xx>5% при трафике>1rps. Это стартовые пороги для демо, их нужно настроить под реальные SLO. Alert rules видны в Prometheus; Alertmanager и отправка сообщений внешним адресатам не настроены.

## Проверка

```sh
make check-observability
# Либо без запуска всего стека:
docker compose --env-file .env.example --profile observability config --quiet
# После старта:
docker compose --profile observability exec -T prometheus promtool check config /etc/prometheus/prometheus.yml
docker compose --profile observability exec -T prometheus promtool check rules /etc/prometheus/alerts.yml
```

Go tests проверяют exposition и bounded labels, auth disabled/401/success, замену поддельного Grafana identity header, удаление gateway Authorization, rewrite путей, pgAdmin prefix/host/scheme и доступность OpenAPI без frontend build. PG integration дополнительно проверяет SELECT-only привилегии viewer. Полный smoke должен проверить Swagger assets/Try it out, Basic challenge, pgAdmin browser/Query Tool, четыре UP targets, datasource Grafana и каждый dashboard против настоящего стека. До такого запуска нельзя считать Docker/Grafana runtime приёмку пройденной.

API spec хранится в `internal/app/specifications`. При изменении контрактов обновлять schemas/paths и проверять их OpenAPI validator. Пути внутренних моков не проксируются наружу ради Swagger.

Первым rate/histogram графикам нужны как минимум два scrape (обычно30–45s). До наблюдений latency может показывать No data. При Redis maxmemory=0 memory-utilization не вычисляется; это отсутствие лимита, а не нулевое использование.

Redis по умолчанию ограничен REDIS_MAXMEMORY=256mb и политикой noeviction. При отказе новых cache writes PG остаётся источником готовых ответов; существующие ключи не вытесняются. Лимит подбирается по нагрузке.
