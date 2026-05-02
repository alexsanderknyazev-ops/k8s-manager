# K8s Manager Runbook

## 1) Users get `forbidden` after permission change

Expected behavior: sessions are invalidated on permission updates.

Steps:
- Ask user to log in again.
- Verify permission exists:
  - `GET /api/permissions?subject=<user>`
- Check deny metric:
  - `rbac_denied_total`

## 2) Locked out from `/api/permissions`

Emergency recovery:
- Temporarily set `RBAC_LEGACY_ADMIN_BYPASS=true`.
- Restart deployment.
- Grant explicit permission to admin/operator:
  - subject=`<email-or-username>`, namespace=`*`, resource=`permissions`, verb=`write`
- Set `RBAC_LEGACY_ADMIN_BYPASS=false` back and restart.

## 3) OIDC login failing

Check:
- `OIDC_GOOGLE_CLIENT_ID`
- `OIDC_GOOGLE_CLIENT_SECRET`
- `OIDC_REDIRECT_URL` must match provider config exactly.
- `OIDC_ISSUER` (default `https://accounts.google.com`).
- If configured: `OIDC_ALLOWED_EMAIL_DOMAINS`.

Symptoms and actions:
- `oidc token verify failed`: issuer/audience mismatch or stale JWKS.
- `oidc nonce mismatch`: stale login tab/session, retry login.
- `oidc domain is not allowed`: add domain to `OIDC_ALLOWED_EMAIL_DOMAINS`.

## 4) Validate DB migration state

Run in Postgres:
- `SELECT version, applied_at FROM schema_migrations ORDER BY version;`

Expected:
- version `2` present.

## 5) Postgres not reachable

Check:
- pod status/events in namespace where Postgres runs.
- application logs for connect/ping errors.
- service DNS and credentials (`POSTGRES_DSN` or bootstrap variables).

## 6) High 5xx / auth failures / RBAC deny spike

Use alerts from `deploy/prometheus-alerts-example.yaml`:
- `K8sManagerHighApi5xxRate`
- `K8sManagerHighAuthFailureRate`
- `K8sManagerRBACDenySpike`

First checks:
- recent deploy/config changes,
- OIDC provider availability,
- permission changes in `/api/permissions`,
- Postgres latency/errors.
