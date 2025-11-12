> grok-code-fast

# Helm + K8s для микросервисной архитектуры на Go + ZITADEL SSO + UI8Kit

Задача — построить конструктор админпанелей, где администраторы могут гибко настраивать личные кабинеты для разных CMS (блоги, мультимедиа, интернет-магазины, корпоративные сайты). Ключевой вызов: Избежать избыточности (лишние сущности/поли в БД) и обеспечить vendor-neutrality — чтобы конструктор работал в любой стране, на любых K8s-совместимых облаках (AWS EKS, GCP GKE, Azure AKS, DigitalOcean) или VDS (on-prem k3s), без зависимости от провайдера.

Я опираюсь на наш [roadmap](roadmap.md) и принципы CyberOS/NixOS-like подхода. Конструктор должен быть modular, portable и self-sufficient. Вот правила, которых придерживаться, с объяснениями и примерами.

### 1. **Декларативность и стандартизация: Использовать только стандартные K8s-примитивы и Helm**
   - **Почему:** K8s — универсальный оркестратор, поддерживаемый всеми облаками. Избегай proprietary extensions (e.g., AWS EBS-only volumes) — используй standard PersistentVolumes, Services, Ingress.
   - **Как:** Все конфигурации в Helm-чартах (YAML/Templating). Для разных CMS — feature flags в `values.yaml` (e.g., `enableBlog: true` — добавляет сущности posts/comments; `enableShop: true` — добавляет products/orders).
   - **Правило:** Нет hardcode провайдера. Для storage — generic PVC; для ingress — standard Ingress (с cert-manager для SSL, без cloud-specific ALB).
   - **Пример:** В чарте админпанели: `if .Values.cmsType == "blog" then include blog-entities in Supabase schema`.

### 2. **Модулярность и feature-driven design: Разделять сущности по модулям**
   - **Почему:** Не все CMS нуждаются в одних сущностях (e.g., блог не нужен в категориях products). Избегай monolithic БД — modular schemas в Supabase/PostgreSQL.
   - **Как:** 
     - Микросервисы на Go: Отдельные для blog, shop, media (с общим API Gateway).
     - БД: Row Level Security (RLS) в Supabase для изоляции данных per-tenant. Сущности как add-ons (e.g., "добавить e-commerce" — миграция добавляет таблицы products, без влияния на blog).
     - Конструктор: UI8Kit-based wizard, где админ выбирает модули (drag-and-drop для сущностей).
   - **Правило:** Zero bloat — lazy loading модулей. Обновления через Helm upgrades без downtime.
   - **Пример:** Для интернет-магазина — только сущности products, orders, payments; для блога — posts, tags, media.

### 3. **Vendor-neutrality и multi-cloud compatibility: Абстрагировать инфраструктуру**
   - **Почему:** Зависимость от провайдера (e.g., AWS Lambda) блокирует portability. Нужно работать в любой стране/облаке без VPN/firewall-issues.
   - **Как:**
     - **K8s distros:** Поддерживать managed (EKS/GKE) и self-hosted (k3s on VDS).
     - **Cloud-agnostic tools:** Ingress — Traefik/Nginx; storage — MinIO (S3-compatible) вместо cloud storage; DB — Supabase (self-hosted PostgreSQL).
     - **Конфиги:** Helm values для разных providers (e.g., `cloudProvider: aws` — используй IAM roles; `cloudProvider: generic` — basic auth).
     - **Networking:** No external APIs — все внутри кластера. Для egress — allowlist (как в roadmap).
   - **Правило:** Тестировать на multi-cloud (CI/CD с matrix: AWS, GCP, on-prem). Избегать region-locked services.
   - **Пример:** Админпанель работает на VDS в России (k3s) и AWS в США без изменений.

### 4. **Data portability и self-sufficiency: Локальные данные, encrypted updates**
   - **Почему:** Данные должны быть portable (export/import) и не зависеть от провайдера. Обновления через облако — с шифрованием (как мы обсуждали: BorgBackup с ZITADEL auth).
   - **Как:**
     - **БД:** Supabase schemas как code (migrations in Git). Export data в encrypted dumps.
     - **Updates:** Offline-бандлы или encrypted pulls (client-side decryption). No vendor update APIs.
     - **Backup/restore:** Встроенный сервис с ZITADEL SSO, zero-knowledge в облаке.
   - **Правило:** Airgapped-first — все работает offline; облако только для sync.
   - **Пример:** Админ экспортирует config в JSON, импортирует на новый кластер.

### 5. **Безопасность и compliance: Self-hosted, no external deps**
   - **Почему:** В разных странах (Россия, Китай) — compliance (ФЗ-152, GDPR). Нет зависимости от cloud APIs.
   - **Как:**
     - **Auth:** ZITADEL self-hosted для SSO.
     - **Encryption:** End-to-end (TLS, RLS in DB).
     - **Auditing:** SigNoz для logs, без export.
   - **Правило:** No SaaS deps (e.g., не использовать Stripe для payments — self-hosted альтернативы).
   - **Пример:** В Китае — локальный кластер с allowlist для WeChat.

### 6. **Тестирование и CI/CD: Автоматизированная валидация portability**
   - **Почему:** Гарантировать, что конструктор работает everywhere.
   - **Как:** GitOps с Flux/ArgoCD. Tests: Helm template validation, multi-cloud pipelines.
   - **Правило:** Docs с чек-листами для каждого provider (e.g., "для AWS: установи IAM policy X").
   - **Пример:** CI проверяет чарт на k3s, EKS, GKE.

Эти правила обеспечат vendor-neutral конструктор: гибкий, без избыточности, portable. 