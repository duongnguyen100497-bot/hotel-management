# Backend implementation blueprint

This folder is the planning-only backend design for the Go/PostgreSQL modular monolith. No backend application code, SQL migrations, or frontend changes were created.

The current, consolidated specification is [Implementation Plan](implementation-plan.md). It incorporates the fixed Redis hold, MinIO media, SePay/wallet, SMTP, single-hotel, Dynamic RBAC and reporting requirements.

Read in this order:

1. [Architecture overview](architecture-overview.md) and [frontend requirements analysis](frontend-requirements-analysis.md)
2. [Business domains](business-domain-analysis.md), [module design](module-design.md), [package structure](package-structure.md)
3. [Authentication and dynamic RBAC](authentication-authorization.md)
4. [Database design](database-design.md), [ERD](erd.md), [migration plan](migration-plan.md)
5. [API specification](api-specification.md), [security](security-design.md), [integrations](integration-design.md), [testing](testing-strategy.md)
6. [Roadmap](implementation-roadmap.md) and [open questions](open-questions.md)
7. **[Backend Task Breakdown (`tasks/README.md`)](tasks/README.md)** — bộ 36 task dọc theo nghiệp vụ (1 task = 1 git commit) để triển khai backend theo [implementation-plan.md](implementation-plan.md).

The highest-priority pre-implementation action is resolving the decisions flagged as blocking in the open-question register; the blueprint intentionally avoids silently treating frontend mock values as production policy.
