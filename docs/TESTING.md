# Проверки

```sh
make test-integration
make lint
make frontend-check
```

`make check` объединяет эти проверки и валидацию Compose. Для быстрого прогона Go без БД используйте `make test-race`. Интеграционные тесты запускают отдельные PostgreSQL и Redis и удаляют тестовые volumes после завершения. Для ручного прогона задайте `TEST_DATABASE_URL` и `TEST_REDIS_ADDR` только для тестовых хранилищ.

GitHub Actions проверяет Go с race detector и линтером, фронтенд с Playwright и полный Compose на чистой БД. Ручные наборы покрывают LTV, календарные границы, продления и возвраты. Интеграционные сценарии проверяют сбои источников, атомарную публикацию, Redis fallback и HTTP-кеш.

На запущенном стенде выполните smoke-тест с Node.js:

```sh
node --env-file=.env scripts/smoke.mjs
```

Smoke проверяет снимок, API, встроенный frontend, Swagger, Grafana, Prometheus и доступ к pgAdmin.
