# Эксплуатация

Запуск, параметры `.env`, адреса и просмотр логов описаны в [README](../README.md).

## Деплой

На сервере нужны Docker Engine и Compose 2.23.1+. Перенесите checkout или архив кода и задайте отдельные пароли в `.env`. Для проверки по публичному IP привяжите `ANALYTICS_PORT=0.0.0.0:8080` и разрешите входящий TCP 8080 со своего IP.

Для HTTPS-прокси на том же хосте используйте `ANALYTICS_PORT=127.0.0.1:8080` и `PUBLIC_BASE_URL=https://<домен>`. Upstream — `http://127.0.0.1:8080`. Если proxy в Docker, подключите его к сети Compose `edge` и используйте `http://analytics-service:8080`. Передавайте исходный `Host` и `X-Forwarded-Proto`.

`make deploy` вызывает запуск Compose на текущем Docker-хосте. Если параллельная сборка перегружает ВМ, соберите образы последовательно:

```sh
(
  set -e
  for service in bootstrap users-service subscriptions-service payments-service analytics-service; do
    docker compose --progress=plain build "$service"
  done
  docker compose --profile observability up --no-build -d --wait
)
```

После запуска проверьте `/status`, `/metrics/ltv/summary` и `/docs/` с другого устройства. При обновлении кода сохраняйте volumes и параметры seed.

## Доступ

Панели используют HTTP Basic через analytics. Для смены пароля оператора измените `OPS_PASSWORD` и примените:

```sh
docker compose --profile observability up -d --no-deps --no-build --wait analytics-service
curl -fsS -u ops http://127.0.0.1:8080/grafana/api/health
```

Укажите своё имя в `curl -u`, если меняли `OPS_USERNAME`. Ответ `"database":"ok"` подтверждает доступ.

Для роли `demo_viewer` одной правки `DB_VIEWER_PASSWORD` недостаточно. Выполните `docker compose exec postgres psql -U postgres -d postgres`, затем `\password demo_viewer`. Запишите тот же пароль в `.env`, пересоздайте pgAdmin и обновите файл подключения в его volume:

```sh
docker compose --profile observability up -d --no-deps --force-recreate --wait pgadmin
docker compose --profile observability exec -T pgadmin sh -ec 'umask 077; cp /run/config/pgpass /var/lib/pgadmin/.pgpass && chmod 600 /var/lib/pgadmin/.pgpass'
```

В Query Tool команда `SELECT current_user, current_setting('transaction_read_only');` должна вернуть `demo_viewer` и `on`. Внутренние пароли PostgreSQL и Grafana на существующих volumes также меняются в самих сервисах.

## Мониторинг

Четыре дашборда Grafana показывают HTTP, синхронизацию, состояние сервиса и хранилища. Prometheus собирает пять целей. Откройте `/prometheus/targets` для проверки. `slog` доступен через `docker compose logs`.

Для проверки HTTP-дашборда выберите **Last 5 minutes** и отправьте несколько десятков запросов:

```sh
for i in $(seq 1 60); do
  curl -fsS http://127.0.0.1:8080/metrics/ltv/summary >/dev/null || break
  sleep 1
done
```

Для проверки отказа мока задайте, например, `PAYMENTS_MOCK_500_BPS=10000` и `PAYMENTS_MOCK_429_BPS=0`, затем пересоздайте `payments-service`. После следующего опроса `last_error` должен заполниться, а `snapshot_id` остаться прежним. После восстановления источника успешный опрос обновит снимок.
