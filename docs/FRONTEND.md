# Фронтенд: React, TypeScript, Tailwind и shadcn/ui

Исходники находятся в `frontend/`. Используются React 19.3.1, TypeScript 6.0.3 с `strict`, Vite 8.3.5, Tailwind CSS 4.3.3, shadcn/ui на Radix UI 1.8.1, React Router 7.18.5, TanStack Query 5.105.3 и Recharts 3.10.1. Прямые зависимости закреплены точными версиями, транзитивные — `package-lock.json`. TypeScript выбран в совместимой с ESLint версии; переход на TS 7 без обновления lint toolchain не выполняется.

Компоненты shadcn/ui добавлены официальным CLI 4.23.0 из registry `new-york-v4` и хранятся в `src/components/ui`, вместе с MIT-лицензией. Это исходники проекта: настроены локальный `cn`, плоские поверхности и `wrapperClassName` для NativeSelect. `components.json` сохраняет настройки registry. Основной шрифт — Geist с кириллицей; IBM Plex Mono используется для UUID и кода. Шрифты и OFL-лицензии хранятся локально, CDN для работы интерфейса не нужен.

В production Go отдаёт только скомпилированные файлы из `embed`, фронтенд обращается к тем же HTTP API. Все семь разделов, фильтры и режим предпросмотра сохранены. React Router использует hash-маршруты, поэтому ссылки вида `/#/cohorts` работают при прямом открытии. Состояние формы не зависит от пересоздания DOM при изменении высоты окна; запросами и их отменой управляет TanStack Query.

Обзор сначала показывает LTV и выручку, затем график когорт и сравнение четырёх тарифов. Подробные расчёты продлений открываются отдельно. Цвет используется на данных; карточки, навигация и фильтры остаются нейтральными. Сравнение UTM можно развернуть в диалоге, а нажатие на источник, тариф или когорту открывает соответствующий сегмент. Показатели строятся из API без декоративных процентов роста.

`Ctrl+K` / `Cmd+K` открывает поиск по разделам со стрелками и Enter. На телефоне навигация и фильтры сворачиваются. Кнопка ссылки сохраняет параметры рабочей выборки (`from`, `to`, `plan_id`, `utm_link`, `selection`) в hash URL; полученную ссылку можно открыть в новой вкладке. Preview использует отдельный набор и не изменяет PostgreSQL; ссылка на рабочую выборку в нём отключена. CSV и все служебные панели доступны из интерфейса.

Для разработки достаточно оставить Compose на порту 8080, затем при установленных Node 24.22.1 и npm 12.2.1 выполнить:

```sh
cd frontend
npm ci
npm run dev
```

Откройте `http://localhost:5173`: Vite обновляет React без пересборки Go и проксирует `/api`, `/metrics`, `/status`, `/healthz` на `http://127.0.0.1:8080`. Адрес backend для dev меняется через `API_PROXY_TARGET`; это настройка dev-сервера, а не секрет, попадающий в клиентский bundle. Альтернатива без Node на хосте — `make frontend-dev`, использующий Docker и порт 5173.

Проверки и сборка из каталога `frontend/`:

```sh
npm run typecheck
npm run lint
npm test
npm run build
npm run test:e2e
```

`build` запускает TypeScript и Vite и создаёт `frontend/dist`. Fixture browser-тесты Playwright сами запускают Vite; нужен установленный Chromium (`npx playwright install chromium`). Отдельные live-тесты используют уже работающий Docker-стек без подмены API. Команды описаны в [TESTING.md](TESTING.md).

Без Node на хосте `make frontend-check` выполняет typecheck, lint, unit tests и production build в закреплённом Node Docker-образе. `make frontend-test` и `make frontend-lint` запускают соответствующие части. Linux `node_modules` из этих команд хранятся в отдельном volume и не заменяют зависимости Windows-разработки.

Для локального запуска Go со встроенным UI выполните `make frontend-build` (или Docker build с target `frontend-export`): assets экспортируются в `internal/app/web/dist`, после чего можно собирать Go. Без собранного `dist` Go-пакеты и backend-тесты компилируются; UI такого dev-бинарника отвечает 503. При обычном `docker compose up --build` эта подготовка выполняется автоматически. Генерируемые каталоги исключены из Git и Docker context, а свежий bundle копируется между Docker-стадиями.

Исходники UI находятся в `frontend/`; готовый bundle создаётся Docker-сборкой и в Git не хранится.
