# Проверки

## GitHub Actions

Workflow `.github/workflows/ci.yml` запускается при push в main, pull request и вручную.

1. **Backend:** `go test -race -count=1 ./...` с настоящими PostgreSQL/Redis, затем golangci-lint. Интеграционные тесты создают и удаляют отдельную тестовую БД.
2. **Frontend:** TypeScript, ESLint, Vitest, production build и Playwright на desktop/tablet/phone с фиксированными API-fixtures.
3. **Docker acceptance:** свежий стек из всех сервисов и инструментов, проверка Prometheus, реальных HTTP-контрактов/бонусов/304 и браузерная проверка встроенного frontend против настоящего API.

Go 1.24.4, Node 24.22.1 и npm 12.2.1 совпадают со сборкой Docker. Actions закреплены commit SHA; workflow имеет только contents:read. CI создаёт случайные пароли и свои временные volumes, затем удаляет окружение. Секреты сервера для CI не нужны.

## Локальные проверки

```sh
make test-race
make lint
make frontend-check
```

Эти команды используют Docker. `make check` объединяет race/unit, линтер, frontend и проверку Compose. Для Playwright-fixtures из `frontend/`: `npm ci`, `npx playwright install chromium`, `npm run test:e2e`.

`TEST_DATABASE_URL` и `TEST_REDIS_ADDR` включают Go integration suite. Без TEST_DATABASE_URL соответствующий тест пропускается. Нужна отдельная PostgreSQL-среда с правом CREATE DATABASE/ROLE; не направляйте эти проверки в production.

После запуска полного стека, при установленном Node 24:

```sh
node --env-file=.env scripts/smoke.mjs
cd frontend
npm ci
npx playwright install chromium
cd ..
node --env-file=.env frontend/node_modules/@playwright/test/cli.js test --config frontend/playwright.live.config.ts
```

Live browser-тесты используют API без подмен: разделы, фильтры, Telegram LTV, preview/возврат к снимку, Swagger, Grafana и GET → 304. Файл env задаёт URL и учётную запись оператора. Для отдельного адреса задайте BASE_URL. Генерация preview не меняет рабочую БД.

`scripts/prepare-ci-env.mjs` предназначен для свежего CI checkout: создаёт новый .env с 10 000 пользователей и отказывается перезаписывать существующий файл. Не используйте его для замены своих настроек.

Локальные screenshots, Playwright reports, логи и отчёты сверки не входят в Git. Зелёный Go unit-run не заменяет integration/browser/Compose-проверки.
