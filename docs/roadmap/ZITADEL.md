> gpt-5-high

Я обновлю TODO под упрощённый скоуп (ZITADEL + Supabase + UI8Kit):

- [ ] Revise roadmap for simplified WorkOS-like (ZITADEL+Supabase+UI8Kit)
- [ ] Decide ZITADEL deployment strategy (Cloud vs self-host) and regions
- [ ] Provision Supabase for metadata (tenants, apps, providers, webhooks)
- [ ] Configure ZITADEL orgs/projects/apps via management API
- [ ] Build embeddable auth widget and theming
- [ ] Add provider connectors (Google, GitHub, Microsoft) UI
- [ ] Ship SDKs and quickstarts (Node/Next.js, Go) for OIDC
- [ ] Implement event webhooks for login/user lifecycle
- [ ] Write docs: setup, provider guides, runbooks
- [ ] Package with Docker Compose and Helm charts
- [ ] Release CLI to manage tenants/apps/providers
- [ ] Set up basic monitoring and alerting
- [ ] Publish demo storefront using the widget and SDKs

### Упрощённая дорожная карта (WorkOS‑лайт на ZITADEL + Supabase + UI8Kit)

- Важные допущения
  - ZITADEL выступает IdP (OIDC). Supabase — каталог метаданных: тенанты, приложения, провайдеры, вебхуки, аудит.
  - UI8Kit Core — для админ‑портала и конструктора виджетов.
  - Начинаем в одном регионе; ZITADEL Cloud для скорости, self‑host/Helm готовим параллельно.

### Скоуп MVP
- User Management: организации/тенанты, админ‑пользователи, базовые роли.
- Unified SSO: OIDC авторизация через ZITADEL.
- Провайдеры: подключение Google, GitHub, Microsoft через UI (админ сам регистрирует приложения у провайдеров).
- Конструктор виджетов: вставляемый JS виджет (login/register), базовая тема/настройки, генерация сниппета.
- Регистрация приложений: управление redirect URI, секретами, политиками входа.
- SDK + Quickstarts: Node/Next.js и Go (верификация токенов, сессии).
- События/вебхуки: login success, user created/updated, connector added.
- Документация: гайды по подключению провайдеров, сниппеты, примеры.

### Архитектура и сервисы
- ZITADEL: проекты/приложения per‑tenant; управление через Management API (gRPC/REST).
- Supabase (Postgres): `tenants`, `apps`, `providers`, `webhooks`, `audit_events`, `settings`.
- Сервисы (Go):
  - API Gateway: аутентификация админ‑панели, маршрутизация.
  - Tenants Service: CRUD тенантов/приложений/провайдеров (Supabase).
  - ZITADEL Adapter: создание проектов/клиентов, политики, связывание провайдеров.
  - Widget Builder: генерация/js-delivery виджета, темы.
  - Webhooks Service: подписки клиентов, ретраи, подпись событий.
  - Audit Service: запись админ‑действий/событий в Supabase.
- UI:
  - Admin Portal (React + UI8Kit): мастера подключения провайдеров, управление приложениями, превью виджетов.
  - Demo storefront: минимальный пример интеграции.

### Фазы и вехи

- Нед. 1 — Платформа-скелет
  - Поднять ZITADEL (Cloud), Supabase проект.
  - Бэкенд: API Gateway, Tenants Service, ZITADEL Adapter (create org/project/app).
  - Admin Portal: онбординг тенанта, регистрация приложения, управление redirect URI.
  - Критерии: создать тенанта → приложение → получить client_id/secret → OIDC авторизация работает на демо.

- Нед. 2 — Виджеты и провайдеры
  - Widget Builder: конфиг, темы, код‑сниппет.
  - Провайдеры: мастера для Google/GitHub/Microsoft (пошаговые инструкции, валидация redirect URI).
  - Demo storefront: вставка виджета, вход через Google/GitHub.
  - Критерии: за 10 минут админ подключает провайдера и логинится через виджет на демо.

- Нед. 3 — SDK и события
  - SDKs: Node/Next.js и Go (OIDC verify, middleware).
  - Webhooks: события входа/выхода/профиля, настройка подписок, подпись, ретраи.
  - Аудит: базовый журнал админ‑действий.
  - Критерии: пример backend получает вебхук о входе; SDK‑шаблоны проходят e2e.

- Нед. 4 — Упаковка и эксплуатация
  - Docker Compose (локалка) и Helm (k8s), чек‑листы деплоя.
  - Мониторинг/алертинг минимум, SLA‑дашборд.
  - CLI: создание тенанта/приложения/провайдера из терминала.
  - Критерии: развернуть за <30 мин; интеграция по гайду без чата с нами.

### Минимальные критерии качества
- Онбординг: от регистрации до рабочего SSO за <30 мин.
- Поддержка 3 провайдеров из коробки (Google, GitHub, Microsoft).
- Виджет вставляется одним сниппетом, конфигурируется из портала.
- SDK и Quickstart запускаются одной командой.
- Вебхуки доставляются с ретраями и верифицируемой подписью.

### Риски и решения
- Трение при регистрации у провайдеров: мастера с авто‑валидацией redirect URI и готовыми скриншот‑гайдами.
- Мульти‑тенантность ZITADEL: модель «org → project → app per‑tenant», храним маппинг в Supabase.
- Локалка/прод: начать с ZITADEL Cloud, параллельно готовим self‑host Helm (CockroachDB) для on‑prem.

Сейчас зафиксирую дорожную карту как завершённую и начну задачу по выбору стратегии деплоя ZITADEL (Cloud vs self‑host) и региона.