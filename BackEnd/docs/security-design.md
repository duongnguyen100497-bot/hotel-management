# Security and data protection design

## Required before production

| Area | Control |
|---|---|
| Transport/secrets | TLS only; HSTS; secrets from managed secret store/env injection, never repo; rotation runbook; database TLS and least-privilege application role. |
| Password/session | Argon2id, per-password salt, calibrated memory/time; generic failures; reset token hashes; refresh rotation/reuse detection; Secure HttpOnly SameSite cookie; session revocation and authorization version checks. |
| Authorization | Central middleware plus use-case contextual checks; default deny; DB RBAC lifecycle protections; audit every login, privilege, data-export, payment/refund and sensitive PII access/change. |
| Input/output | Typed DTO validation, request body limits, JSON decode rejection of unknown fields where practical, parameterized SQL/SQLC, output encoding by frontend, allowlisted sort/filter, safe error mapping. |
| PII | Classify customer names/contact/address as confidential; passport/government ID as restricted. Encrypt restricted columns with envelope encryption/key ID, store deterministic keyed hash only for duplicate check; mask in lists/logs/audit; never log credentials/tokens/document values. |
| Financial integrity | Server totals/snapshots; append-only payments/refunds, idempotency, least privilege, immutable invoice lines; reconciliation/report definitions approved. |
| Abuse | Reverse-proxy and application limits: login/reset/register/support/booking; per-IP+identifier counters; CAPTCHA only after abuse evidence; upload quotas; monitoring/alerts. |
| Uploads | Direct-to-private object storage via scoped short-lived URL; validate declared/actual MIME, magic bytes, dimensions/size, malware scan before publish; random keys, no executable SVG unless sanitized; authorization on download. |
| Browser | CORS exact allowlist; CSRF double-submit/origin validation for cookie refresh/logout and state requests; CSP/security headers; no bearer token in localStorage. |
| Operations | dependency/vulnerability scan, migration access separation, backups encrypted/tested, restore drill, structured redacted logs, metrics/alerts, incident and retention runbooks. |

## Sensitive-data retention

Retention periods need legal/business sign-off. Proposed controls: keep accounting invoices/payments/audit according to local law (do not assume period); archive inactive operational data; expire reset tokens quickly; retain session/security evidence for a documented incident window; delete/anonymize customer PII only after retention eligibility while preserving non-identifying financial referential records. Encrypt backups and test recovery.

## Later enhancements

MFA for protected administrators, device/session notifications, SIEM integration, field-level search tokenization, independent penetration test, WAF rules, and payment-provider PCI scope assessment. They do not replace baseline controls above.
