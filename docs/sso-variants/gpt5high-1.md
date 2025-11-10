I'll draft a phased roadmap and spin up an implementation TODO list so we can track progress from architecture to MVP and beyond. Then I'll present the plan and set the next concrete decision task in progress.

[1 tool called]

I’ve drafted the roadmap and defined the architecture and data flows. I’ll mark those tasks complete and set “Decide Logto OSS deployment strategy and regions” as the next active task.

[1 tool called]

### Северная звезда и MVP
- **Цель**: SSO и IAM как сервис для админов сайтов, строгая GDPR/PII защита, ПД в стране гражданства, активность — где выгодно.
- **MVP**: 1) Логин через Logto OSS (OIDC), 2) CMS хранит только псевдоним `user_ref` без PII, 3) Админ‑кабинет с управлением клиентами (сайтами), ролями и отзывом доступа, 4) Consent (opt‑in) и выдача только выбранных атрибутов, 5) Шифрование PII на стороне SSO, 6) DSR: Export/Delete.

### Архитектура (высокоуровнево)
- **IdP**: Logto OSS (OpenID Connect/OAuth 2.0) + Management API.
- **PII сервис**: отдельное API/ресурс‑сервер, хранит PII в Postgres, реализует поле‑уровневое шифрование и GDPR операции.
- **Хранилище**: Postgres (предпочтительно Supabase как managed) — per‑tenant/region; RLS и схемы для изоляции.
- **Ключи**: KMS (AWS KMS / GCP KMS / HashiCorp Vault). Envelope encryption, ротация ключей.
- **Админ‑портал**: Next.js/TypeScript, RBAC/ABAC, управление клиентами (сайтами), конструктор authorizations/scopes, аудит, DSR панели.
- **CMS интеграции**: SDK (OIDC Client), middleware, примеры для Next.js/Node, PHP/Laravel.
- **Аудит/События**: неизменяемые логи (объектное хранилище с lock/retention), OpenTelemetry, SIEM интеграция.

### Потоки данных (ключевые)
- **Регистрация/Логин**: CMS редиректит в Logto; возвращается `id_token` с pairwise `sub` (см. ниже), CMS сохраняет только `user_ref` и тех.метаданные.
- **Активность в CMS**: все события привязаны к `user_ref`. PII не сохраняется.
- **Доступ к PII**: по opt‑in/consent Logto выдает `access_token` со scopes; CMS получает атрибуты через UserInfo/PII API и не кэширует (или хранит только зашифровано ключом SSO).
- **Ревокация**: админ сайта или пользователь отзывает consent/клиент — токены немедленно инвалидируются, вебхуки уведомляют CMS.
- **GDPR DSR**: Export — выгрузка из PII сервиса; Delete — удаление/развязывание `user_ref` (активность в CMS остаётся, но не де‑псевдонимизируема).

### Идентификаторы и де‑идентификация
- **Внутренний ID**: `user_global_id` в PII сервисе.
- **Псевдоним в CMS**: `user_ref` = pairwise `sub` из OIDC (per‑site/sector). Если Logto OSS не поддержит из коробки — fallback: HMAC(`user_global_id`, site_salt).
- **Sector Identifier**: уникальный per‑site, чтобы `sub` был разный для разных сайтов.
- **Нулевая PII в CMS**: имя/фамилия/email/телефон — не хранятся. Если бизнесу нужно хранить — только шифротекст с ключом, недоступным CMS.

### Шифрование и ключи
- **Envelope**: KMS Master Key -> Data Keys (per‑tenant, per‑attribute class). AES‑GCM для значений, AAD: tenant/site/user.
- **Ротация**: версии ключей, фоновая ре‑шифрация, double‑write при ротации.
- **Поиск**: email — детерминированное шифрование + blind index (SHA‑256 с солью) для equality lookup (минимизировать).
- **Изоляция**: минимум — per‑tenant ключи; лучше — per‑region per‑tenant.

### Consent и sharing
- **Scopes→Claims**: явная карта атрибутов к scope (e.g. `profile:name`, `contact:email`).
- **Purpose/TTL**: фиксируем цель обработки и срок действия consent.
- **Default‑deny**: без opt‑in CMS видит только `user_ref` и технические клеймы.
- **UI**: пользователь сам выбирает, чем делиться; всё логируется.

### Data residency и мульти‑тенантность
- **Вариант А (MVP)**: один регион (например, EU‑Central) — один Logto OSS, один Supabase проект; потом масштабируем.
- **Вариант B (продакшн)**: per‑tenant per‑region Supabase проект; Logto OSS инстанс(ы) в том же регионе; центральный админ‑портал управляет инвентарём (регистром тенантов и регионов).
- **Изоляция**: минимум — отдельные схемы + RLS; лучше — отдельные базы/проекты per‑tenant.

### Модель данных (ядро PII)
- `users_pii(id, assurance_level, created_at, ...)`
- `identifiers(user_id, type, value_enc, value_blind_idx)`
- `consents(user_id, client_id, scopes, purpose, ttl, status, audit)`
- `keys(key_id, tenant_id, version, material_wrapped, state)`
- `audit_events(id, actor, subject, action, time, hash_chain)`

### Digital Passport (клеймы)
- Обязательные: `sub` (pairwise), `iss`, `aud`, `iat`, `exp`
- Контекст: `user_ref`, `passport.v`, `consents`, `assurance_level`, `tenant`, `site`
- Валидация CMS: OIDC Discovery + JWKS, проверка `aud`/`iss`/подписи/exp.

Пример клеймов (упрощённо):

```json
{
  "iss": "https://id.your-tenant.eu",
  "aud": "cms-shop-123",
  "sub": "ppid_4b6c... (pairwise)",
  "user_ref": "ppid_4b6c...",
  "passport": { "v": 1, "consents": ["profile:name"], "assurance_level": "high" },
  "iat": 1731200000,
  "exp": 1731203600
}
```

### Нефункциональные требования
- **Безопасность**: WAF, rate‑limits, mTLS между сервисами, секреты в KMS/Secrets Manager.
- **Надёжность**: point‑in‑time recovery, регламент бэкапов, DR‑план.
- **Наблюдаемость**: метрики, трассировки, централизованные логи, алерты.
- **GDPR**: DPIA, RoPA, DPA с провайдерами, SCCs, breach policy, data minimization.

### Поэтапный план и критерии
- **Нед. 1–2 (MVP‑α)**:
  - Развернуть Logto OSS + Supabase (1 регион).
  - Pairwise `sub`/`user_ref` на CMS‑демо.
  - Админ‑портал: регистрация сайта (OAuth client), базовый RBAC.
  - Критерии: CMS не хранит PII, логин/логаут, отзыв клиента работает.
- **Нед. 3–4 (MVP‑β)**:
  - Envelope encryption (KMS), PII API, Consent UI/flows.
  - DSR: Export/Delete, аудиторские логи (append‑only).
  - Критерии: экспорт ZIP, удаление развязывает активность, аудит непротиворечив.
- **Нед. 5–6 (Pilot)**:
  - Перенос в выбранный регион прод, мониторинг/бэкапы/DR.
  - SDK для 1–2 CMS (Next.js/Node, PHP).
  - Критерии: пилот с реальным магазином и платёжкой, ревокация/инцидент‑дрилл.

### Риски и развилки
- **Logto OSS мульти‑тенантность**: если ограничена — запускать per‑tenant инстансы; управлять через мета‑слой.
- **Pairwise sub**: если не доступно — HMAC‑derivation c секретом per‑site.
- **Search по PII**: ограничить до минимально необходимого; документировать trade‑offs.
- **Data residency**: стоимость множества проектов/инстансов vs. схемы+RLS.
- **Человеческий фактор**: DSR и аудит — автоматизировать, исключать ручные ошибки.

### Учебный путь (минимум)
- День 1: OIDC/OAuth2 потоки (Auth Code + PKCE), JWT/JWKS.
- День 2: Supabase Postgres основы: схемы, RLS, миграции.
- День 3: Logto OSS: установка, клиенты, клеймы/скоупы.
- День 4: Next.js/TypeScript, интеграция OIDC Client, защищённые роуты.
- День 5: KMS и envelope encryption (Node libs, ключевые практики).
- День 6: GDPR DSR, аудит, цепочки хэшей, object lock.

### Что дальше (решение, чтобы не тормозить)
- Рекомендую стартовать с **Варианта А**: один регион EU‑Central (или другой под ваш пилот), один Supabase проект, один Logto OSS инстанс, один демо‑CMS.
- Если у вас уже есть целевые рынки/страны, дайте список — на 5–6 неделе включим per‑tenant per‑region.

— — —

- **Следующий активный шаг**: “Decide Logto OSS deployment strategy and regions”.
  - Нужна ваша вводная: пилотный регион/страна гражданства первой аудитории и целевой CMS (например, Next.js магазин или WooCommerce). Если нет предпочтений — беру EU‑Central (Франкфурт) как дефолт для MVP.

Коротко об изменениях:
- Сформулировал целевую модель, разделение PII/активности, идентификаторы, шифрование, consent, residency и DSR.
- Разбил на этапы, критерии готовности, риски и учебный план.
- В TODO отмечены выполненные пункты (дорожная карта, архитектура), следующий — выбор стратегии развертывания Logto и регионов.