# Go package structure and dependency enforcement

```text
BackEnd/
├── cmd/api/main.go
├── internal/platform/
│   ├── config/             # typed env config and validation
│   ├── db/                 # pgx pool, transaction runner, SQLC wiring
│   ├── http/               # router, middleware, error encoder
│   ├── auth/               # principal extraction, token/cookie mechanics
│   ├── logging/ observability/ clock/ id/
├── internal/modules/
│   ├── identity/{domain,application,delivery/http,persistence/postgres}
│   ├── people/{domain,application,delivery/http,persistence/postgres}
│   ├── property/{domain,application,delivery/http,persistence/postgres}
│   ├── reservations/{domain,application,delivery/http,persistence/postgres}
│   ├── servicecatalog/{domain,application,delivery/http,persistence/postgres}
│   ├── stayops/{domain,application,delivery/http,persistence/postgres}
│   ├── billing/{domain,application,delivery/http,persistence/postgres}
│   ├── support/{domain,application,delivery/http,persistence/postgres}
│   ├── reporting/{application,delivery/http,persistence/postgres}
│   └── audit/{application,persistence/postgres}
├── db/{migration,query,seed}
├── api/openapi.yaml
└── docs/
```

`domain` has entities/value objects/invariants and ports only. `application` has commands, queries, use cases and carefully chosen cross-module interfaces. `delivery/http` owns request/response DTOs and handlers. `persistence/postgres` owns SQLC adapters and only module-owned SQL. `cmd` composes dependencies; it contains no domain decisions.

Allowed imports: delivery → application/domain; application → own domain + platform ports + explicitly exported other-module application interfaces; persistence → own application/domain + platform DB; composition → all. Prohibited: module-to-module persistence import, domain-to-HTTP/SQL import, handlers-to-repositories, reporting writes to source tables, and generic `shared/models` business package. Enforce with code review and a Go import-boundary test/linter in CI.
