# K8s Manager

Веб-интерфейс для управления Kubernetes-кластером: дашборд, поды, деплойменты, логи, метрики, порт-форвардинг.

## Требования

- Go 1.22+ (или Docker)
- Доступ к кластеру Kubernetes (kubeconfig)

## Запуск

**По умолчанию** приложение поднимает PostgreSQL в кластере (если `POSTGRES_DSN` не задан), подключается к нему и создаёт по одному пользователю на роль: **admin** и **viewer**. Второй инстанс БД не создаётся — если Postgres уже развёрнут в namespace, используется он.

```bash
# Минимальный запуск: Postgres в кластере, логин admin/secret и viewer/viewer
go run .
```

Откройте в браузере: http://localhost:8080. Логин по умолчанию: **admin** / **secret** (или **viewer** / **viewer** для только чтения).

**Без bootstrap** (один пользователь из env, БД не поднимается):

```bash
export BOOTSTRAP_POSTGRES_IN_CLUSTER=false
export AUTH_USER=admin
export AUTH_PASSWORD=secret
go run .
```

Пароль в виде bcrypt-хэша: `AUTH_PASSWORD_HASH='$2a$10$...'` (сгенерировать: `go run ./cmd/hashpass/main.go`).

### Аутентификация через PostgreSQL (логин, пароль, роли)

Если задан `POSTGRES_DSN` (или `DATABASE_URL`), пользователи и права берутся из БД. Таблица `users`: `username`, `password_hash` (bcrypt), `role` (`admin` или `viewer`). Роль **viewer** — только чтение (мутирующие запросы к API блокируются); **admin** — полный доступ (если не включён глобальный `READ_ONLY`).

При первом старте с пустой БД можно создать первого админа переменными `FIRST_ADMIN_USER` и `FIRST_ADMIN_PASSWORD`. Дополнительных пользователей можно добавлять в таблицу `users` (поля `username`, `password_hash` — bcrypt, `role`: `admin` или `viewer`).

```bash
export POSTGRES_DSN="postgres://user:pass@localhost:5432/k8smanager?sslmode=disable"
export FIRST_ADMIN_USER=admin
export FIRST_ADMIN_PASSWORD=secret
go run .
```

**Docker Compose** (поднимает Postgres и приложение):

```bash
docker compose up -d
# Логин: admin, пароль: secret (из FIRST_* в docker-compose.yml)
```

### Postgres в кластере (по умолчанию)

Если `POSTGRES_DSN` не задан, при старте приложение проверяет, есть ли уже развёрнутый Postgres в namespace (`BOOTSTRAP_POSTGRES_NAMESPACE`, по умолчанию `default`). Если да — подключается к нему и не создаёт второй. Если нет — создаёт Secret, ConfigMap, PVC, Service и Deployment с образом `postgres:16-alpine`, ждёт готовности пода и подключается. В пустую БД создаётся по одному пользователю на роль: admin (из `FIRST_ADMIN_*`) и viewer (из `FIRST_VIEWER_*`).

```bash
# Опционально: свой пароль БД и пользователи (по умолчанию admin/secret, viewer/viewer)
export POSTGRES_PASSWORD=mypass
export FIRST_ADMIN_USER=admin
export FIRST_ADMIN_PASSWORD=secret
export FIRST_VIEWER_USER=viewer
export FIRST_VIEWER_PASSWORD=viewer
go run .
```

Отключить bootstrap и использовать один логин из env: `BOOTSTRAP_POSTGRES_IN_CLUSTER=false` и `AUTH_USER` / `AUTH_PASSWORD`. Подключение к развёрнутой БД идёт по адресу `postgres.<namespace>.svc.cluster.local:5432`; при запуске вне кластера приложение поднимает port-forward на localhost.

## Docker

```bash
docker build -t k8s-manager .
docker run --rm -p 8080:8080 -v "$HOME/.kube:/root/.kube:ro" -e AUTH_USER=admin -e AUTH_PASSWORD=secret k8s-manager
```

### Dev-кластер (Minikube + Prometheus + Grafana + Postgres)

Удобный сценарий для тестов: **одна команда** поднимает minikube, Prometheus и Grafana, затем запускает приложение — оно само поднимает Postgres в namespace `default` и создаёт пользователей admin/viewer.

**Требования:** установленные [minikube](https://minikube.sigs.k8s.io/) и `kubectl`. Для стабильного старта minikube поднимается с `--memory=4g --cpus=2`. На Mac (arm64) сначала пробуется драйвер **qemu2** (часто стабильнее Docker) — установка: `brew install qemu`. Задать драйвер вручную: `MINIKUBE_DRIVER=docker go run . dev-cluster run`.

**Запустить dev-среду (кластер + приложение):**

```bash
go run . dev-cluster run
# или короче:
make dev-run
```

Тестовые пользователи (создаются автоматически в `dev-cluster run`):

| Логин | Пароль | Права |
|---|---|---|
| `admin` | `secret` | полный доступ (role=admin) |
| `viewer` | `viewer` | только чтение (`*/*/read`) |
| `dev` | `devpass` | `default/deployments/write`, `default/pods/read`, `default/services/read` |
| `ops` | `opspass` | `default/pods/write`, `default/deployments/read` |

После старта откройте http://localhost:7777 (или `DEV_PORT`, по умолчанию 7777; логин **admin** / **secret**).

**Grafana** (дашборды K8s Manager, логин **admin** / **admin**):

```bash
kubectl -n k8s-manager port-forward svc/grafana 3000:3000
# http://localhost:3000
```

**Prometheus:** `kubectl -n monitoring port-forward svc/prometheus-server 9090:80`

**Остановить dev-среду:**

```bash
go run . dev-cluster stop
# или:
make dev-stop
```

**Перезапустить кластер** (остановить и поднять заново):

```bash
go run . dev-cluster stop && go run . dev-cluster run
# или:
make dev-restart
```

Отдельно: `go run . dev-cluster start` — только minikube + Prometheus + Grafana; затем `go run .` — только приложение. Манифесты: `deploy/dev-cluster/prometheus.yaml`, `deploy/grafana-provisioning.yaml`.

## Переменные окружения

| Переменная           | Описание                              | По умолчанию        |
|----------------------|----------------------------------------|---------------------|
| `KUBECONFIG`         | Путь к kubeconfig                     | `$HOME/.kube/config` |
| `PORT`               | Порт HTTP-сервера                     | `8080`              |
| `POSTGRES_DSN` / `DATABASE_URL` | Подключение к PostgreSQL; при задании логин/пароль/роли из БД | — |
| `BOOTSTRAP_POSTGRES_IN_CLUSTER` | Развернуть PostgreSQL в кластере при старте, если `POSTGRES_DSN` не задан. По умолчанию **включено**, если DSN пустой. Отключить: `false` или `0`. Если Postgres уже есть в namespace — второй не создаётся. | `true` при пустом DSN |
| `BOOTSTRAP_POSTGRES_NAMESPACE` | Namespace для развёрнутого Postgres | `default` |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Учётные данные БД при bootstrap | user: `k8smanager`, password: `secret`, db: `k8smanager` |
| `FIRST_ADMIN_USER` / `FIRST_ADMIN_PASSWORD` | Логин/пароль пользователя с ролью admin при пустой БД (один на роль) | `admin` / `secret` |
| `FIRST_VIEWER_USER` / `FIRST_VIEWER_PASSWORD` | Логин/пароль пользователя с ролью viewer при пустой БД (один на роль) | `viewer` / `viewer` |
| `AUTH_USER`          | Логин (один пользователь из env, если нет POSTGRES_DSN)       | —                   |
| `AUTH_PASSWORD`      | Пароль в открытом виде                | —                   |
| `AUTH_PASSWORD_HASH` | Пароль в виде bcrypt-хэша (вместо AUTH_PASSWORD) | —          |
| `OIDC_GOOGLE_CLIENT_ID` | Google OIDC Client ID (включает OIDC auth) | — |
| `OIDC_GOOGLE_CLIENT_SECRET` | Google OIDC Client Secret | — |
| `OIDC_REDIRECT_URL` | Callback URL (например `https://your-host/api/auth/oidc/callback`) | — |
| `OIDC_ISSUER` | OIDC issuer URL (по умолчанию Google: `https://accounts.google.com`) | `https://accounts.google.com` |
| `OIDC_ALLOWED_EMAIL_DOMAINS` | Разрешённые домены email (через запятую), например `company.com,subsidiary.io` | — |
| `OIDC_LOGOUT_URL` | URL logout у IdP (если задан, logout редиректит туда) | — |
| `RBAC_LEGACY_ADMIN_BYPASS` | Временная совместимость: роль `admin` обходит granular RBAC (`true/1`) | `false` |
| `OIDC_TEST_ADMIN_EMAIL` | Тестовый email для seed permissions (`*/*/*`) | — |
| `OIDC_TEST_VIEWER_EMAIL` | Тестовый email для seed permissions (`*/*/read`) | — |
| `READ_ONLY`          | Глобальный режим только чтение (`true`/`1`) | — |
| `COOKIE_SECURE`      | Флаг `Secure` у cookie сессии (при HTTPS — `true`) | `false` |
| `LOG_LEVEL`          | Уровень логов: `debug`, `info`, `warn`, `error` | `info` |
| `LOG_FORMAT`         | Формат логов: `text` или `json` | `text` |
| `RATE_LIMIT_LOGIN_PER_MIN` | Лимит запросов на `/api/login` с одного IP в минуту | `10` |
| `RATE_LIMIT_API_PER_MIN`   | Лимит запросов на `/api/*` с одного IP в минуту (`0` = без лимита) | `300` |

Аутентификация включается через OIDC (`OIDC_GOOGLE_CLIENT_ID`, `OIDC_GOOGLE_CLIENT_SECRET`, `OIDC_REDIRECT_URL`) или legacy login/password (`AUTH_*`). В OIDC-режиме парольный логин отключён, а `id_token` проверяется по issuer/audience (JWKS). Для granular RBAC используются записи в таблице `user_permissions` (seed через `OIDC_TEST_ADMIN_EMAIL` и `OIDC_TEST_VIEWER_EMAIL`). Сессия в cookie, 24 ч. Эндпоинты `/api/health` и `/metrics` доступны без авторизации.

## Runbook (коротко)

- **`forbidden` после выдачи/отзыва прав:** текущие сессии пользователя инвалидируются автоматически; перелогиниться.
- **OIDC вход отклонён по домену:** проверь `OIDC_ALLOWED_EMAIL_DOMAINS` и домен email claim.
- **Потерян доступ к `/api/permissions`:** временно включи `RBAC_LEGACY_ADMIN_BYPASS=true`, выдай право `permissions/write`, затем выключи bypass.
- **Проверка миграции:** в БД должна быть таблица `schema_migrations` с версией `2`.

## API

Списки поддерживают параметр `?limit=N` (поды, деплойменты, события), по умолчанию до 500. OpenAPI-спека: `GET /api/docs` (JSON).

### Архитектура

- **Фронт:** серверный рендеринг (Go templates), Bootstrap 5, боковая навигация в стиле OpenShift (зелёная тема). Порты и действия (Port-forward, удаление) доступны в контексте страниц (Pods, Deployments, Config), а не в главном меню.
- **Бэкенд:** Gin, middleware: Request ID (`X-Request-Id`), Security headers, CSRF, rate limit, Prometheus metrics, audit log. Обращения к Kubernetes API — через `kubernetes.Interface` и metrics client; при необходимости таймауты задаются через `context.WithTimeout` в хендлерах.
- **Деплой:** манифесты в `deploy/` (Deployment, Service, Ingress, Secret, PDB). Пример правил алертов Prometheus — `deploy/prometheus-alerts-example.yaml`.

## Возможности

- **Dashboard** — обзор подов, деплойментов, нод, сервисов, метрик
- **Деплой** — быстрый деплой по шаблону (NGINX, Redis, PostgreSQL и др.) или из своего образа: форма → Deployment + опционально Service
- **Pods** — список, логи, YAML, удаление
- **Deployments** — масштабирование, рестарт, редактирование YAML
- **Services, ConfigMaps, Secrets** — просмотр и YAML
- **Метрики** — CPU/RAM подов и нод (нужен [Metrics Server](https://github.com/kubernetes-sigs/metrics-server))
- **Порт-форвардинг** — проброс портов из подов на localhost
- **Логи в реальном времени** — стриминг по WebSocket
- **Granular RBAC** — управление правами в UI `/ui/permissions` и API `/api/permissions` (таблица `user_permissions`: `subject`, `namespace`, `resource`, `verb`)

## Makefile

```bash
make build      # сборка
make test       # тесты
make run        # go run
make run-auth   # с AUTH_USER/AUTH_PASSWORD
make docker-build
make docker-run
make hashpass   # генерация bcrypt-хэша пароля
make migrate-status  # статус SQL-миграций (POSTGRES_DSN обязателен)
make migrate-up      # применить все pending миграции
make migrate-down    # откатить последнюю миграцию
make dev-run         # поднять minikube+инфру и запустить приложение
make dev-start       # поднять только minikube+инфру
make dev-stop        # остановить dev-кластер
make dev-restart     # stop + run
make deploy-in-cluster  # образ + Deployment + Ingress в текущий кластер
make deploy-undeploy    # удалить Deployment/Ingress/RBAC из кластера
```

### Деплой в кластер одной командой (как под + Ingress)

Сборка образа, RBAC, Deployment и Ingress (`deploy/ingress-dev.yaml`, HTTP без TLS):

```bash
make deploy-in-cluster
```

После деплоя (minikube + Docker driver):

```bash
echo "127.0.0.1 k8s-manager.local" | sudo tee -a /etc/hosts   # один раз
minikube tunnel   # отдельный терминал
```

Откройте **http://k8s-manager.local** (не `https://`). Логин: **admin** / **secret**.

Переопределение образа: `IMAGE=myregistry/k8s-manager:v1 make deploy-in-cluster`.

Отдельный migration tooling лежит в `cmd/migrate/main.go`, SQL-файлы — в `migrations/` (`*.up.sql` и `*.down.sql`).
Для migration CLI есть advisory lock (защита от параллельного запуска), а timeout на DB-операции настраивается через `MIGRATE_TIMEOUT` (по умолчанию `2m`, можно переопределить флагом `-timeout`, например `-timeout=5m`).

## Манифесты

Все манифесты в каталоге `deploy/`:

- **`deploy/dev-cluster/prometheus.yaml`** — Prometheus в namespace `monitoring` (скрейп `/metrics` приложения).
- **`deploy/grafana-provisioning.yaml`** — Grafana в `k8s-manager` с дашбордами; поднимается в `dev-cluster start` и `make deploy-in-cluster`.
- **`deploy/deployment.yaml`**, **`deploy/rbac.yaml`**, **`deploy/ingress.yaml`** — K8s Manager (namespace `k8s-manager`).
- **`deploy/postgres-market.yaml`** — ручной деплой PostgreSQL в namespace `market` (опционально; для аутентификации обычно достаточно bootstrap в `default`).
- **`deploy/k8s-manager-default.yaml`** — альтернативный деплой приложения в namespace `default` (SA, RBAC, Deployment, Service, Ingress).
- **`deploy/metrics-server.yaml`**, **`deploy/metrics-server-fixed.yaml`** — Metrics Server для CPU/RAM в UI (в minikube: `minikube addons enable metrics-server`).
- **`deploy/RUNBOOK.md`** — операционный runbook: OIDC/RBAC/permissions recovery, миграции и типовые инциденты.
- **`deploy/grafana-k8s-manager-overview.json`** — готовый operational dashboard для Grafana (RPS, 5xx, auth failures, RBAC denies).
- **`deploy/grafana-k8s-manager-slo.json`** — SLO dashboard (availability SLI, error budget, burn indicators).
- **`deploy/grafana-provisioning.yaml`** — Grafana + provisioning через ConfigMap (datasource + auto-loaded dashboards).
- **`deploy/grafana-provisioning-prod.yaml`** — production-вариант Grafana: admin из Secret + Ingress TLS.

### Быстрый старт Grafana provisioning

```bash
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/grafana-provisioning.yaml
kubectl -n k8s-manager port-forward svc/grafana 3000:3000
```

Открыть: `http://localhost:3000` (по умолчанию `admin/admin`).  
Datasource настроен на `http://prometheus-server.monitoring.svc.cluster.local` — при другом адресе Prometheus поменяй `url` в `deploy/grafana-provisioning.yaml`.

### Production-вариант Grafana (Secret + TLS Ingress)

```bash
kubectl apply -f deploy/deployment.yaml
kubectl -n k8s-manager create secret generic grafana-admin-auth \
  --from-literal=admin-user=admin \
  --from-literal=admin-password='REPLACE_WITH_STRONG_PASSWORD' \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f deploy/grafana-provisioning-prod.yaml
```

Что важно перед запуском:
- Secret `grafana-admin-auth` должен быть создан заранее (команда выше);
- убедиться, что TLS secret `grafana-tls` существует в `k8s-manager`;
- настроить DNS/hosts для `grafana.k8s-manager.local`.

## Тесты

```bash
go test ./api/ ./internal/sessionstore/ ./internal/audit/ ./internal/auth/
```

---

## Прод или только dev?

Приложение можно использовать **в проде**: деплой через `deploy/deployment.yaml` в namespace `k8s-manager`, Ingress с TLS, аутентификация через PostgreSQL или Secret, rate limit, CSRF, метрики Prometheus. Ниже — что уже есть и что настроить перед продакшеном.

## Готовность к продакшену

**Реализовано:**

- **HTTPS** — в `deploy/ingress.yaml` включены TLS и редирект на HTTPS. Нужен Secret `k8s-manager-tls` (cert-manager или свой сертификат).
- **Сессии при replicas > 1** — при использовании PostgreSQL сессии и журнал аудита хранятся в БД (таблицы `sessions`, `audit_log`), поэтому несколько реплик работают без sticky session. Без Postgres сессии — в памяти (один инстанс). В Ingress при необходимости можно включить sticky session.
- **Rate limiting** — лимит на логин (`RATE_LIMIT_LOGIN_PER_MIN`, по умолчанию 10) и на API (`RATE_LIMIT_API_PER_MIN`, по умолчанию 300). `0` = без лимита для API.
- **CSRF** — токен выдаётся при логине (cookie), проверяется для POST/PUT/DELETE к `/api/*`. Фронт отправляет заголовок `X-CSRF-Token` через `apiFetch`.
- **Метрики Prometheus** — эндпоинт `GET /metrics` (без авторизации), счётчики `http_requests_total{method,path,status}`.
- **Дашборды Grafana “из коробки”** — JSON для импорта в Grafana: `deploy/grafana-k8s-manager-overview.json` и `deploy/grafana-k8s-manager-slo.json`.
- **Логи** — `LOG_LEVEL` (debug/info/warn/error), `LOG_FORMAT` (text/json).
- **Секреты** — пример в `deploy/secret-example.yaml`; в `deploy/deployment.yaml` закомментированы env из Secret.
- **Безопасность пода** — в Deployment задан `securityContext.allowPrivilegeEscalation: false`.

**Рекомендации:** хранить логин/пароль в Secret и подставлять через `valueFrom.secretKeyRef`; при доступе через HTTPS задать `COOKIE_SECURE=true`.

### Оценка готовности к продакшену

| Критерий | Оценка | Комментарий |
|----------|--------|--------------|
| **Безопасность** | ✅ Готово | Аутентификация (Postgres/Secret), CSRF, rate limit, заголовки (X-Frame-Options, X-Content-Type-Options и др.), cookie HttpOnly + SameSite Lax, опция Secure. RBAC и securityContext в деплое. |
| **HTTPS / TLS** | ✅ Готово | Ingress с TLS и редиректом на HTTPS; нужен Secret с сертификатом (cert-manager или свой). |
| **Масштабирование** | ✅ Готово при Postgres | При `POSTGRES_DSN` сессии и аудит в БД — replicas > 1 без sticky session. Без Postgres сессии в памяти (один инстанс). |
| **Наблюдаемость** | ✅ Готово | `/metrics` для Prometheus, логи (уровень и формат), request_id, аудит запросов (при Postgres — в БД; иначе в памяти с лимитом). |
| **Надёжность** | ✅ Готово | Liveness/readiness на `/api/health`, лимиты CPU/RAM у пода, graceful shutdown. |
| **Секреты и конфиг** | ✅ Готово | Примеры через Kubernetes Secret; пароли не в коде. |

**Итог:** приложение **пригодно для продакшена** при типичных условиях: один или несколько инстансов за Ingress с TLS, доступ к кластеру по kubeconfig/ServiceAccount. При использовании Postgres пользователи, сессии и журнал аудита хранятся в БД — подходит для нескольких реплик без потери сессий при рестарте.
