# Как увидеть изменения в Grafana

## Запуск нагрузки

Из папки репозитория, когда основной стек уже работает:

```sh
docker compose --profile demo run --build --rm demo-traffic
```

Это 3 минуты, до 10 GET-запросов/сек, 4 worker. Если конфигурация хранится отдельно, добавьте обычный `--env-file путь/к/файлу` перед `--profile`.

Пример с явной длительностью и интенсивностью, одинаковый для PowerShell и shell:

```powershell
docker compose --profile demo run --build --rm demo-traffic -duration 3m -rps 10 -concurrency 4
```

При наличии Make: `make demo-traffic` или `make demo-traffic DEMO_DURATION=5m DEMO_RPS=10`. Для отдельного env-файла задайте `COMPOSE_ENV=путь/к/файлу`.

CLI флаги: url (или TARGET_URL), duration не больше 15m, rps 1–50, concurrency 1–16. Запрос имеет timeout 5s. Если workers заняты, новые тики пропускаются, а не накапливаются в очереди. Ctrl+C корректно отменяет запросы. Логи каждые 5 секунд показывают число успешных запросов, 4xx/5xx, network errors, skipped_busy и среднюю задержку. Параметры и авторизация tools не попадают в URL/логи.

## Куда смотреть

1. Откройте `/grafana/d/subscope-overview?from=now-5m&to=now&refresh=15s` рядом с терминалом.
2. Через 2–4 scrape (примерно 30–60 секунд) на «Запросы к сервисам» появится плато до 10 req/s у analytics-service.
3. «API аналитики · p95» показывает задержки настоящих endpoints. «Кеш» сначала может показать miss/PG, затем Redis hit при повторении тех же запросов.
4. В HTTP и Go смотрите RPS, p95/p99, in-flight, CPU и память. В Sync/Cache — результаты кеша и длительность построения ответов.
5. После завершения генератора RPS падает. На графике остаётся история подъёма/спада.

Интервал scrape и автообновления — 15s, а rate/histogram требуют нескольких наблюдений. При отсутствии HTTP-трафика процент ошибок может показывать «Нет трафика». Не нужно создавать искусственные 500 ради заполнения графика.

## Что именно делает генератор

Вызывает summary, by-plan, by-utm, retention двух тарифов, dashboard с несколькими страницами/месяцами и UTM-фильтры. Количество вариантов ограничено; бесконечных уникальных cache keys нет. Исходные users/subscriptions/payments не меняются. Обычная работа API может добавлять responses в PostgreSQL и кеш Redis.

Число пользователей и платежей в Grafana относится к **опубликованному live-снимку** и остаётся прежним от HTTP-нагрузки. Preview в интерфейсе создаёт временный набор для frontend и тоже не переписывает live-набор. Источники в обычном режиме опрашиваются каждые 10 минут — там изменяются возраст снимка и счётчики синхронизации.

Для дополнительной CPU-нагрузки можно вручную открыть генератор в приложении и создать preview на 25/50/100 тысяч пользователей: изменение памяти/latency будет видно в Go-панелях, а размер live-набора не подменится preview-данными.

## pgAdmin

Откройте `/db/`, войдите обычной учётной записью ops. Разверните Демо → Аналитика подписок — PostgreSQL → Databases → ваша база → Schemas. Подключение и пароль уже настроены, роль demo_viewer имеет SELECT.

В Query Tool можно выполнить:

```sql
SELECT current_database(), current_user;

SELECT 'users' AS entity, count(*) FROM users.users
UNION ALL
SELECT 'subscriptions', count(*) FROM subscriptions.subscriptions
UNION ALL
SELECT 'payments', count(*) FROM payments.payments;

SELECT current_id, last_success, next_sync, last_error
FROM analytics.state;

SELECT count(*) AS cached_responses
FROM analytics.responses;
```

Последний запрос показывает сохранённые cache responses; их число может вырасти при новых фильтрах и снимках. Счётчик source payments от обычного GET не увеличивается.
